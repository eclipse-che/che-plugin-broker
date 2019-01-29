//
// Copyright (c) 2019 Red Hat, Inc.
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
	"strconv"

	"github.com/eclipse/che-go-jsonrpc"
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
	rand common.Random
}

// NewBroker creates Che VS Code extension broker instance
func NewBroker() *Broker {
	return &Broker{
		Broker:  common.NewBroker(),
		ioUtil:  files.New(),
		Storage: storage.New(),
		client:  &http.Client{},
		rand : common.NewRand(),
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
	if meta.Attributes == nil || meta.Attributes["extension"] == "" {
		return fmt.Errorf("VS Code extension field 'extension' is missing in description of plugin %s:%s", meta.ID, meta.Version)
	}
	url := meta.Attributes["extension"]
	image := meta.Attributes["containerImage"]
	if image == "" {
		return fmt.Errorf("VS Code extension field 'containerImage' is missing in description of plugin %s:%s", meta.ID, meta.Version)
	}

	workDir, err := b.ioUtil.TempDir("", "vscode-extension-broker")
	if err != nil {
		return err
	}

	archivePath := filepath.Join(workDir, "pluginArchive")
	unpackedPath := filepath.Join(workDir, "plugin")

	// Download an archive
	b.PrintDebug("Downloading archive '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, archivePath)
	err = b.download(url, archivePath, meta)
	if err != nil {
		return err
	}

	// Unzip it
	b.PrintDebug("Unzipping archive '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, unpackedPath)
	err = b.ioUtil.Unzip(archivePath, unpackedPath)
	if err != nil {
		return err
	}

	pj, err := b.getPackageJSON(unpackedPath)
	if err != nil {
		return err
	}

	return b.injectRemotePlugin(meta, unpackedPath, image, pj)
}

func (b *Broker) injectRemotePlugin(meta model.PluginMeta, unpackedPath string, image string, pj *model.PackageJSON) error {
	pluginFolderPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s", meta.ID, meta.Version))
	b.PrintDebug("Copying VS Code extension '%s:%s' from '%s' to '%s'", meta.ID, meta.Version, unpackedPath, pluginFolderPath)
	err := b.ioUtil.CopyResource(unpackedPath, pluginFolderPath)
	if err != nil {
		return err
	}
	tooling := &model.ToolingConf{
		Containers: []model.Container{*b.ContainerConfig(image)},
	}
	b.addPortToTooling(tooling, pj)
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
	defer ignoreError(resp.Body.Close())
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

func ignoreError(err error) {}

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

func (b *Broker) ContainerConfig(image string) *model.Container {
	c := model.Container{
		Name:  "theiapluginsidecar" + b.rand.String(6),
		Image: image,
		Volumes: []model.Volume{
			{
				Name:      "projects",
				MountPath: "/projects",
			},
			{
				Name:      "plugins",
				MountPath: "/plugins",
			},
		},
	}
	return &c
}

// AddPortToTooling adds to tooling everything needed to start Theia remote plugin:
// - Random port to the container (one and only)
// - Endpoint matching the port
// - Environment variable THEIA_PLUGIN_ENDPOINT_PORT to the container with the port as value
// - Environment variable that start from THEIA_PLUGIN_REMOTE_ENDPOINT_ and ends with
// plugin publisher and plugin name taken from packageJson and replacing all
// chars matching [^a-z_0-9]+ with a dash character
func (b *Broker) addPortToTooling(toolingConf *model.ToolingConf, pj *model.PackageJSON) {
	port := b.rand.IntFromRange(4000, 10000)
	sPort := strconv.Itoa(port)
	endpointName := b.rand.String(10)
	var re = regexp.MustCompile(`[^a-zA-Z_0-9]+`)
	prettyID := re.ReplaceAllString(pj.Publisher+"_"+pj.Name, `_`)
	theiaEnvVar1 := "THEIA_PLUGIN_REMOTE_ENDPOINT_" + prettyID
	theiaEnvVarValue := "ws://" + endpointName + ":" + sPort

	toolingConf.Containers[0].Ports = append(toolingConf.Containers[0].Ports, model.ExposedPort{ExposedPort: port})
	toolingConf.Endpoints = append(toolingConf.Endpoints, model.Endpoint{
		Name:       endpointName,
		Public:     false,
		TargetPort: port,
	})
	toolingConf.Containers[0].Env = append(toolingConf.Containers[0].Env, model.EnvVar{Name: "THEIA_PLUGIN_ENDPOINT_PORT", Value: sPort})
	toolingConf.WorkspaceEnv = append(toolingConf.WorkspaceEnv, model.EnvVar{Name: theiaEnvVar1, Value: theiaEnvVarValue})
}
