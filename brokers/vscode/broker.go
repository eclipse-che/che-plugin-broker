//
// Copyright (c) 2012-2018 Red Hat, Inc.
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
	"math/rand"
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

// VSCodeExtensionBroker is used to process VS Code extensions to run them as Che plugins
type VSCodeExtensionBroker struct {
	common.Broker
	ioUtil  files.IoUtil
	Storage *storage.Storage
}

// NewBroker creates Che VS Code extension broker instance
func NewBroker() *VSCodeExtensionBroker {
	return &VSCodeExtensionBroker{
		common.NewBroker(),
		files.New(),
		storage.New(),
	}
}

// Start executes plugins metas processing and sends data to Che master
func (broker *VSCodeExtensionBroker) Start(metas []model.PluginMeta) {
	if ok, status := broker.Storage.SetStatus(model.StatusStarted); !ok {
		m := fmt.Sprintf("Starting broker in state '%s' is not allowed", status)
		broker.PubFailed(m)
		broker.PrintFatal(m)
	}
	broker.PubStarted()
	broker.PrintInfo("Started Plugin Broker")

	broker.PrintPlan(metas)

	broker.PrintInfo("Starting plugins processing")
	for _, meta := range metas {
		err := broker.processPlugin(meta)
		if err != nil {
			broker.PubFailed(err.Error())
			broker.PrintFatal(err.Error())
		}
	}

	if ok, status := broker.Storage.SetStatus(model.StatusDone); !ok {
		err := fmt.Sprintf("Setting '%s' broker status failed. Broker has '%s' state", model.StatusDone, status)
		broker.PubFailed(err)
		broker.PrintFatal(err)
	}

	plugins, err := broker.Storage.Plugins()
	if err != nil {
		broker.PubFailed(err.Error())
		broker.PrintFatal(err.Error())
	}
	pluginsBytes, err := json.Marshal(plugins)
	if err != nil {
		broker.PubFailed(err.Error())
		broker.PrintFatal(err.Error())
	}

	broker.PrintInfo("All plugins have been successfully processed")
	broker.PubDone(string(pluginsBytes))
	broker.CloseConsumers()
}

// PushEvents sets given tunnel as consumer of broker events.
func (broker *VSCodeExtensionBroker) PushEvents(tun *jsonrpc.Tunnel) {
	broker.Broker.PushEvents(tun, model.BrokerStatusEventType, model.BrokerResultEventType, model.BrokerLogEventType)
}

func (broker *VSCodeExtensionBroker) processPlugin(meta model.PluginMeta) error {
	broker.PrintDebug("Stared processing plugin '%s:%s'", meta.ID, meta.Version)
	if meta.Attributes == nil || meta.Attributes["extension"] == "" {
		return fmt.Errorf("VS Code extension field 'extension' is missing in description of plugin %s:%s", meta.ID, meta.Version)
	}
	url := meta.Attributes["extension"]
	image := meta.Attributes["container-image"]
	if image == "" {
		return fmt.Errorf("VS Code extension field 'container-image' is missing in description of plugin %s:%s", meta.ID, meta.Version)
	}

	workDir, err := broker.ioUtil.TempDir("", "vscode-extension-broker")
	if err != nil {
		return err
	}

	archivePath := filepath.Join(workDir, "pluginArchive")
	unpackedPath := filepath.Join(workDir, "plugin")

	// Download an archive
	broker.PrintDebug("Downloading archive '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, archivePath)
	err = broker.download(url, archivePath, meta)
	if err != nil {
		return err
	}

	// Unzip it
	broker.PrintDebug("Unzipping archive '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, unpackedPath)
	err = broker.ioUtil.Unzip(archivePath, unpackedPath)
	if err != nil {
		return err
	}

	pj, err := broker.getPackageJSON(unpackedPath)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}
	return broker.injectRemotePlugin(meta, unpackedPath, image, pj)
}

func (broker *VSCodeExtensionBroker) getPackageJSON(pluginFolder string) (*packageJSON, error) {
	packageJSONPath := filepath.Join(pluginFolder, "extension", "package.json")
	broker.PrintDebug("Reading package.json of VS Code extension from '%s'", packageJSONPath)
	f, err := ioutil.ReadFile(packageJSONPath)
	if err != nil {
		return nil, err
	}
	pj := &packageJSON{}
	err = json.Unmarshal(f, pj)
	return pj, err
}

func (broker *VSCodeExtensionBroker) injectRemotePlugin(meta model.PluginMeta, archiveFolder string, image string, pj *packageJSON) error {
	pluginFolderPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s", meta.ID, meta.Version))
	broker.PrintDebug("Copying VS Code extension '%s:%s' from '%s' to '%s'", meta.ID, meta.Version, archiveFolder, pluginFolderPath)
	err := broker.ioUtil.CopyResource(archiveFolder, pluginFolderPath)
	if err != nil {
		return err
	}
	tooling := &model.ToolingConf{
		Containers: []model.Container{*containerConfig(image)},
	}
	broker.addPortToTooling(tooling, pj)
	return broker.Storage.AddPlugin(&meta, tooling)
}

func containerConfig(image string) *model.Container {
	c := model.Container{
		Name:  "vscodeextsidecar" + randomNumberAsString(),
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

func (broker *VSCodeExtensionBroker) addPortToTooling(toolingConf *model.ToolingConf, pj *packageJSON) {
	port := findPort()
	sPort := strconv.Itoa(port)
	endpointName := "port" + sPort
	var re = regexp.MustCompile(`[^a-z_0-9]+`)
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

func randomNumberAsString() string {
	port := findPort()
	return strconv.Itoa(port)
}

func findPort() int {
	return 4000 + rand.Intn(6000)
}

func (broker *VSCodeExtensionBroker) download(extension string, dest string, meta model.PluginMeta) error {
	response, err := fetchExtensionInfo(extension, meta)
	if err != nil {
		return err
	}

	URL, err := findAssetURL(response, meta)
	if err != nil {
		return err
	}

	err = broker.ioUtil.Download(URL, dest)
	return err
}

func fetchExtensionInfo(extension string, meta model.PluginMeta) ([]byte, error) {
	re, err := regexp.Compile(`^vscode:extension/(.*)`)
	if err != nil {
		return nil, fmt.Errorf("VS Code extension id '%s' parsing failed for plugin %s:%s", extension, meta.ID, meta.Version)
	}
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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VS Code extension downloading failed %s:%s. Error: %s", meta.ID, meta.Version, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("VS Code extension downloading failed %s:%s. Status: %q", meta.ID, meta.Version, resp.StatusCode)
	}

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("VS Code extension downloading failed %s:%s. Error: %s", meta.ID, meta.Version, err)
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
