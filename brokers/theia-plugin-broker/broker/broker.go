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

package broker

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
	broker *common.Broker
}

// NewBroker creates Che Theia plugin broker instance
func NewBroker() *TheiaPluginBroker {
	return &TheiaPluginBroker{common.NewBroker()}
}

// Start executes plugins metas processing and sends data to Che master
func (broker *TheiaPluginBroker) Start(metas []model.PluginMeta) {
	if ok, status := storage.SetStatus(model.StatusStarted); !ok {
		m := fmt.Sprintf("Starting broker in state '%s' is not allowed", status)
		broker.broker.PubFailed(m)
		broker.broker.PrintFatal(m)
	}
	broker.broker.PubStarted()
	broker.broker.PrintInfo("Started Plugin Broker")

	// Do not do cleaning.
	// Since we can start several brokers in parallel they should not concurrently clean up resources.
	// Instead of that we need to move cleaning to another phase.

	broker.broker.PrintPlan(metas)

	broker.broker.PrintInfo("Starting plugins processing")
	for _, meta := range metas {
		err := broker.processPlugin(meta)
		if err != nil {
			broker.broker.PubFailed(err.Error())
			broker.broker.PrintFatal(err.Error())
		}
	}

	if ok, status := storage.SetStatus(model.StatusDone); !ok {
		err := fmt.Sprintf("Setting '%s' broker status failed. Broker has '%s' state", model.StatusDone, status)
		broker.broker.PubFailed(err)
		broker.broker.PrintFatal(err)
	}

	plugins, err := storage.Plugins()
	if err != nil {
		broker.broker.PubFailed(err.Error())
		broker.broker.PrintFatal(err.Error())
	}
	pluginsBytes, err := json.Marshal(plugins)
	if err != nil {
		broker.broker.PubFailed(err.Error())
		broker.broker.PrintFatal(err.Error())
	}

	broker.broker.PrintInfo("All plugins have been successfully processed")
	broker.broker.PubDone(string(pluginsBytes))
	broker.broker.CloseConsumers()
}

// PushEvents sets given tunnel as consumer of broker events.
func (broker *TheiaPluginBroker) PushEvents(tun *jsonrpc.Tunnel) {
	broker.broker.PushEvents(tun, model.BrokerStatusEventType, model.BrokerResultEventType, model.BrokerLogEventType)
}

func (broker *TheiaPluginBroker) processPlugin(meta model.PluginMeta) error {
	broker.broker.PrintDebug("Stared processing plugin '%s:%s'", meta.ID, meta.Version)
	url := meta.URL

	workDir, err := ioutil.TempDir("", "theia-plugin-broker")
	if err != nil {
		return err
	}

	archivePath := filepath.Join(workDir, "pluginArchive")
	unpackedPath := filepath.Join(workDir, "plugin")

	// Download an archive
	broker.broker.PrintDebug("Downloading archive '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, archivePath)
	err = files.Download(url, archivePath)
	if err != nil {
		return err
	}

	// Unzip it
	broker.broker.PrintDebug("Unzipping archive '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, unpackedPath)
	err = files.Unzip(archivePath, unpackedPath)
	if err != nil {
		return err
	}

	pj, err := broker.getPackageJSON(unpackedPath)
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

func (broker *TheiaPluginBroker) getPackageJSON(pluginFolder string) (*packageJson, error) {
	packageJSONPath := filepath.Join(pluginFolder, "package.json")
	broker.broker.PrintDebug("Reading package.json of Theia plugin from '%s'", packageJSONPath)
	f, err := ioutil.ReadFile(packageJSONPath)
	if err != nil {
		return nil, err
	}
	pj := &packageJson{}
	err = json.Unmarshal(f, pj)
	return pj, err
}

func (broker *TheiaPluginBroker) getPluginImage(pj *packageJson) (string, error) {
	if pj.Engines.CheRuntimeContainer != "" {
		return pj.Engines.CheRuntimeContainer, nil
	}
	return "", nil
}

func (broker *TheiaPluginBroker) injectTheiaFile(meta model.PluginMeta, archivePath string) error {
	broker.broker.PrintDebug("Copying Theia plugin '%s:%s'", meta.ID, meta.Version)
	pluginPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s.theia", meta.ID, meta.Version))
	err := files.CopyFile(archivePath, pluginPath)
	if err != nil {
		return err
	}
	tooling := &model.ToolingConf{}
	return storage.AddPlugin(&meta, tooling)
}

func (broker *TheiaPluginBroker) injectTheiaRemotePlugin(meta model.PluginMeta, archiveFolder string, image string, pj *packageJson) error {
	pluginFolderPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s", meta.ID, meta.Version))
	broker.broker.PrintDebug("Copying Theia remote plugin '%s:%s' from '%s' to '%s'", meta.ID, meta.Version, archiveFolder, pluginFolderPath)
	err := files.CopyResource(archiveFolder, pluginFolderPath)
	if err != nil {
		return err
	}
	tooling := &model.ToolingConf{
		Containers: []model.Container{*containerConfig(image)},
	}
	broker.addPortToTooling(tooling, pj)
	return storage.AddPlugin(&meta, tooling)
}

func containerConfig(image string) *model.Container {
	c := model.Container{
		Name:  "theiaPluginSidecar" + randomNumberAsString(),
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

func (broker *TheiaPluginBroker) addPortToTooling(toolingConf *model.ToolingConf, pj *packageJson) {
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
