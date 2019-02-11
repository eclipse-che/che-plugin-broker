//
// Copyright (c) 2018-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package vscode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"

	"github.com/eclipse/che-go-jsonrpc"
	"github.com/eclipse/che-plugin-broker/brokers/theia"
	"github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/files"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
)

const marketplace = "https://marketplace.visualstudio.com/_apis/public/gallery/extensionquery"
const bodyFmt = `{"filters":[{"criteria":[{"filterType":7,"value":"%s"}],"pageNumber":1,"pageSize":1,"sortBy":0, "sortOrder":0 }],"assetTypes":["Microsoft.VisualStudio.Services.VSIXPackage"],"flags":131}`
const assetType = "Microsoft.VisualStudio.Services.VSIXPackage"

// Broker is used to process VS Code extensions to run them as Che plugins
type Broker struct {
	common.Broker
	ioUtil  files.IoUtil
	Storage *storage.Storage
	client  *http.Client
	rand    common.Random
}

// NewBroker creates Che VS Code extension broker instance
func NewBroker() *Broker {
	return &Broker{
		Broker:  common.NewBroker(),
		ioUtil:  files.New(),
		Storage: storage.New(),
		client:  &http.Client{},
		rand:    common.NewRand(),
	}
}

// Start executes plugins metas processing and sends data to Che master
func (b *Broker) Start(metas []model.PluginMeta) {
	b.PubStarted()
	b.PrintInfo("Started VS Code Plugin Broker")

	b.PrintPlan(metas)

	b.PrintInfo("Starting VS Code extensions processing")
	for _, meta := range metas {
		err := b.processPlugin(meta)
		if err != nil {
			b.PubFailed(err.Error())
			b.PrintFatal(err.Error())
		}
	}

	plugins, err := b.Storage.Plugins()
	if err != nil {
		b.PubFailed(err.Error())
		b.PrintFatal(err.Error())
	}
	pluginsBytes, err := json.Marshal(plugins)
	if err != nil {
		b.PubFailed(err.Error())
		b.PrintFatal(err.Error())
	}

	b.PrintInfo("All plugins have been successfully processed")
	result := string(pluginsBytes)
	b.PrintDebug(result)
	b.PubDone(result)
	b.CloseConsumers()
}

// PushEvents sets given tunnel as consumer of broker events.
func (b *Broker) PushEvents(tun *jsonrpc.Tunnel) {
	b.Broker.PushEvents(tun, model.BrokerStatusEventType, model.BrokerResultEventType, model.BrokerLogEventType)
}

func (b *Broker) processPlugin(meta model.PluginMeta) error {
	b.PrintDebug("Stared processing plugin '%s:%s'", meta.ID, meta.Version)

	url := meta.URL
	extension := ""
	if meta.Attributes != nil {
		extension = meta.Attributes["extension"]
	}

	if url == "" && extension == "" {
		return fmt.Errorf("Neither 'extension' no 'url' attributes found in VS Code extension description of the plugin %s:%s", meta.ID, meta.Version)
	} else if url != "" && extension != "" {
		return fmt.Errorf("VS Code extension description of the plugin %s:%s might contain either 'extension' or 'url' attributes, but both of them are found", meta.ID, meta.Version)
	}

	workDir, err := b.ioUtil.TempDir("", "vscode-extension-broker")
	if err != nil {
		return err
	}

	archivePath := filepath.Join(workDir, "pluginArchive")

	// Download an archive
	if url != "" {
		b.PrintDebug("Downloading archive '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, archivePath)
		err = b.ioUtil.Download(url, archivePath)
		if err != nil {
			return err
		}
	} else {
		b.PrintDebug("Downloading extension '%s' for plugin '%s:%s' to '%s'", extension, meta.ID, meta.Version, archivePath)
		err = b.download(extension, archivePath, meta)
		if err != nil {
			return err
		}
	}

	image := meta.Attributes["containerImage"]
	if image == "" {
		// regular plugin
		return b.injectLocalPlugin(meta, archivePath)
	}
	// remote plugin
	return b.injectRemotePlugin(meta, image, archivePath, workDir)
}

func (b *Broker) injectLocalPlugin(meta model.PluginMeta, archivePath string) error {
	b.PrintDebug("Copying VS Code extension '%s:%s'", meta.ID, meta.Version)
	pluginPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s.vsix", meta.ID, meta.Version))
	err := b.ioUtil.CopyFile(archivePath, pluginPath)
	if err != nil {
		return err
	}
	tooling := &model.ToolingConf{}
	return b.Storage.AddPlugin(&meta, tooling)
}

func (b *Broker) injectRemotePlugin(meta model.PluginMeta, image string, archivePath string, workDir string) error {
	// Unzip it
	unpackedPath := filepath.Join(workDir, "plugin")
	b.PrintDebug("Unzipping archive '%s' for plugin '%s:%s' to '%s'", archivePath, meta.ID, meta.Version, unpackedPath)
	err := b.ioUtil.Unzip(archivePath, unpackedPath)
	if err != nil {
		return err
	}

	pj, err := b.getPackageJSON(unpackedPath)
	if err != nil {
		return err
	}

	pluginFolderPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s", meta.ID, meta.Version))
	b.PrintDebug("Copying VS Code extension '%s:%s' from '%s' to '%s'", meta.ID, meta.Version, unpackedPath, pluginFolderPath)
	err = b.ioUtil.CopyResource(unpackedPath, pluginFolderPath)
	if err != nil {
		return err
	}
	tooling := theia.GenerateSidecarTooling(image, *pj, b.rand)
	return b.Storage.AddPlugin(&meta, tooling)
}

func (b *Broker) download(extension string, dest string, meta model.PluginMeta) error {
	response, err := b.fetchExtensionInfo(extension, meta)
	if err != nil {
		return err
	}

	URL, err := findAssetURL(response, meta)
	if err != nil {
		return err
	}

	return b.ioUtil.Download(URL, dest)
}

func (b *Broker) fetchExtensionInfo(extension string, meta model.PluginMeta) ([]byte, error) {
	re := regexp.MustCompile(`^vscode:extension/(.*)`)
	groups := re.FindStringSubmatch(extension)
	if len(groups) != 2 {
		return nil, fmt.Errorf("VS Code extension id '%s' parsing failed for plugin %s:%s", extension, meta.ID, meta.Version)
	}
	extName := groups[1]
	body := []byte(fmt.Sprintf(bodyFmt, extName))
	req, err := http.NewRequest("POST", marketplace, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("VS Code extension id '%s' fetching failed for plugin %s:%s. Error: %s", extension, meta.ID, meta.Version, err)
	}
	req.Header.Set("Accept", "application/json;api-version=3.0-preview.1")
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VS Code extension downloading failed %s:%s. Error: %s", meta.ID, meta.Version, err)
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("VS Code extension downloading failed %s:%s. Error: %s", meta.ID, meta.Version, err)
	}
	if resp.StatusCode != 200 {
		errMsg := "VS Code extension downloading failed %s:%s. Status: %v. Body: " + string(body)
		return nil, fmt.Errorf(errMsg, meta.ID, meta.Version, resp.StatusCode)
	}

	return body, nil
}

func findAssetURL(response []byte, meta model.PluginMeta) (string, error) {
	obj := &marketplaceResponse{}
	err := json.Unmarshal(response, obj)
	if err != nil {
		return "", fmt.Errorf("Failed to parse VS Code extension marketplace response for plugin %s:%s", meta.ID, meta.Version)
	}
	switch {
	case len(obj.Results) == 0,
		len(obj.Results[0].Extensions) == 0,
		len(obj.Results[0].Extensions[0].Versions) == 0,
		len(obj.Results[0].Extensions[0].Versions[0].Files) == 0:

		return "", fmt.Errorf("Failed to parse VS Code extension marketplace response for plugin %s:%s", meta.ID, meta.Version)
	}
	for _, f := range obj.Results[0].Extensions[0].Versions[0].Files {
		if f.AssetType == assetType {
			return f.Source, nil
		}
	}
	return "", fmt.Errorf("VS Code extension archive information is not found in marketplace response for plugin %s:%s", meta.ID, meta.Version)
}

func (b *Broker) getPackageJSON(pluginFolder string) (*model.PackageJSON, error) {
	packageJSONPath := filepath.Join(pluginFolder, "extension", "package.json")
	f, err := ioutil.ReadFile(packageJSONPath)
	if err != nil {
		return nil, err
	}
	pj := &model.PackageJSON{}
	err = json.Unmarshal(f, pj)
	return pj, err
}
