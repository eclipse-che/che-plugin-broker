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

package theia

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/eclipse/che-go-jsonrpc"
	"github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/files"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
)

// TheiaPluginBroker is used to process Theia .theia and remote plugins
type TheiaPluginBroker struct {
	common.Broker
	ioUtil  files.IoUtil
	storage *storage.Storage
}

// NewBroker creates Che Theia plugin broker instance
func NewBroker() *TheiaPluginBroker {
	return &TheiaPluginBroker{
		common.NewBroker(),
		files.New(),
		storage.New(),
	}
}

// Start executes plugins metas processing and sends data to Che master
func (broker *TheiaPluginBroker) Start(metas []model.PluginMeta) {
	if ok, status := broker.storage.SetStatus(model.StatusStarted); !ok {
		m := fmt.Sprintf("Starting broker in state '%s' is not allowed", status)
		broker.PubFailed(m)
		broker.PrintFatal(m)
	}
	broker.PubStarted()
	broker.PrintInfo("Started Theia Plugin Broker")

	broker.PrintPlan(metas)

	broker.PrintInfo("Starting Theia plugins processing")
	for _, meta := range metas {
		err := broker.processPlugin(meta)
		if err != nil {
			broker.PubFailed(err.Error())
			broker.PrintFatal(err.Error())
		}
	}

	if ok, status := broker.storage.SetStatus(model.StatusDone); !ok {
		err := fmt.Sprintf("Setting '%s' broker status failed. Broker has '%s' state", model.StatusDone, status)
		broker.PubFailed(err)
		broker.PrintFatal(err)
	}

	plugins, err := broker.storage.Plugins()
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
func (broker *TheiaPluginBroker) PushEvents(tun *jsonrpc.Tunnel) {
	broker.Broker.PushEvents(tun, model.BrokerStatusEventType, model.BrokerResultEventType, model.BrokerLogEventType)
}

func (broker *TheiaPluginBroker) processPlugin(meta model.PluginMeta) error {
	broker.PrintDebug("Stared processing plugin '%s:%s'", meta.ID, meta.Version)
	url := meta.URL

	workDir, err := broker.ioUtil.TempDir("", "theia-plugin-broker")
	if err != nil {
		return err
	}

	archivePath := filepath.Join(workDir, "pluginArchive")
	unpackedPath := filepath.Join(workDir, "plugin")

	// Download an archive
	broker.PrintDebug("Downloading archive '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, archivePath)
	err = broker.ioUtil.Download(url, archivePath)
	if err != nil {
		return err
	}

	// Unzip it
	broker.PrintDebug("Unzipping archive '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, unpackedPath)
	err = broker.ioUtil.Unzip(archivePath, unpackedPath)
	if err != nil {
		return err
	}

	pj, err := GetPackageJSON(unpackedPath)
	if err != nil {
		return err
	}

	pluginImage, err := broker.getPluginImage(pj)
	if err != nil {
		return err
	}
	if pluginImage == "" {
		// regular plugin
		return broker.injectTheiaFile(meta, archivePath)
	}
	// remote plugin
	return broker.injectTheiaRemotePlugin(meta, unpackedPath, pluginImage, pj)
}

func GetPackageJSON(pluginFolder string) (*PackageJSON, error) {
	packageJSONPath := filepath.Join(pluginFolder, "package.json")
	f, err := ioutil.ReadFile(packageJSONPath)
	if err != nil {
		return nil, err
	}
	pj := &PackageJSON{}
	err = json.Unmarshal(f, pj)
	return pj, err
}

func (broker *TheiaPluginBroker) getPluginImage(pj *PackageJSON) (string, error) {
	if pj.Engines.CheRuntimeContainer != "" {
		return pj.Engines.CheRuntimeContainer, nil
	}
	return "", nil
}

func (broker *TheiaPluginBroker) injectTheiaFile(meta model.PluginMeta, archivePath string) error {
	broker.PrintDebug("Copying Theia plugin '%s:%s'", meta.ID, meta.Version)
	pluginPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s.theia", meta.ID, meta.Version))
	err := broker.ioUtil.CopyFile(archivePath, pluginPath)
	if err != nil {
		return err
	}
	tooling := &model.ToolingConf{}
	return broker.storage.AddPlugin(&meta, tooling)
}

func (broker *TheiaPluginBroker) injectTheiaRemotePlugin(meta model.PluginMeta, archiveFolder string, image string, pj *PackageJSON) error {
	pluginFolderPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s", meta.ID, meta.Version))
	broker.PrintDebug("Copying Theia remote plugin '%s:%s' from '%s' to '%s'", meta.ID, meta.Version, archiveFolder, pluginFolderPath)
	err := broker.ioUtil.CopyResource(archiveFolder, pluginFolderPath)
	if err != nil {
		return err
	}
	tooling := &model.ToolingConf{
		Containers: []model.Container{*ContainerConfig(image)},
	}
	AddPortToTooling(tooling, pj)
	return broker.storage.AddPlugin(&meta, tooling)
}

func ContainerConfig(image string) *model.Container {
	c := model.Container{
		Name:  "theiapluginsidecar" + GetRndNumberAsString(),
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
func AddPortToTooling(toolingConf *model.ToolingConf, pj *PackageJSON) {
	port := GetRndPort()
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

// GetRndNumberAsString returns stringified random port from range 4000-6000
func GetRndNumberAsString() string {
	port := GetRndPort()
	return strconv.Itoa(port)
}

// GetRndPort returns random port from range 4000-6000
func GetRndPort() int {
	return 4000 + rand.Intn(6000)
}
