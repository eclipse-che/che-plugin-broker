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

// Broker is used to process .theia and remote plugins
type Broker struct {
	common.Broker
	ioUtil  files.IoUtil
	storage *storage.Storage
}

// NewBroker creates Che Theia plugin broker instance
func NewBroker() *Broker {
	return &Broker{
		Broker:  common.NewBroker(),
		ioUtil:  files.New(),
		storage: storage.New(),
	}
}

// Start executes plugins metas processing and sends data to Che master
func (b *Broker) Start(metas []model.PluginMeta) {
	if ok, status := b.storage.SetStatus(model.StatusStarted); !ok {
		m := fmt.Sprintf("Starting broker in state '%s' is not allowed", status)
		b.PubFailed(m)
		b.PrintFatal(m)
	}
	b.PubStarted()
	b.PrintInfo("Started Theia Plugin Broker")

	b.PrintPlan(metas)

	b.PrintInfo("Starting Theia plugins processing")
	for _, meta := range metas {
		err := b.processPlugin(meta)
		if err != nil {
			b.PubFailed(err.Error())
			b.PrintFatal(err.Error())
		}
	}

	if ok, status := b.storage.SetStatus(model.StatusDone); !ok {
		err := fmt.Sprintf("Setting '%s' broker status failed. Broker has '%s' state", model.StatusDone, status)
		b.PubFailed(err)
		b.PrintFatal(err)
	}

	plugins, err := b.storage.Plugins()
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

	workDir, err := b.ioUtil.TempDir("", "theia-plugin-broker")
	if err != nil {
		return err
	}

	archivePath := filepath.Join(workDir, "pluginArchive")
	unpackedPath := filepath.Join(workDir, "plugin")

	// Download an archive
	b.PrintDebug("Downloading archive '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, archivePath)
	err = b.ioUtil.Download(url, archivePath)
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

	pluginImage, err := b.getPluginImage(pj)
	if err != nil {
		return err
	}
	if pluginImage == "" {
		// regular plugin
		return b.injectTheiaFile(meta, archivePath)
	}
	// remote plugin
	return b.injectTheiaRemotePlugin(meta, unpackedPath, pluginImage, pj)
}

func (b *Broker) getPackageJSON(pluginFolder string) (*PackageJSON, error) {
	packageJSONPath := filepath.Join(pluginFolder, "package.json")
	f, err := ioutil.ReadFile(packageJSONPath)
	if err != nil {
		return nil, err
	}
	pj := &PackageJSON{}
	err = json.Unmarshal(f, pj)
	return pj, err
}

func (b *Broker) getPluginImage(pj *PackageJSON) (string, error) {
	if pj.Engines.CheRuntimeContainer != "" {
		return pj.Engines.CheRuntimeContainer, nil
	}
	return "", nil
}

func (b *Broker) injectTheiaFile(meta model.PluginMeta, archivePath string) error {
	b.PrintDebug("Copying Theia plugin '%s:%s'", meta.ID, meta.Version)
	pluginPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s.theia", meta.ID, meta.Version))
	err := b.ioUtil.CopyFile(archivePath, pluginPath)
	if err != nil {
		return err
	}
	tooling := &model.ToolingConf{}
	return b.storage.AddPlugin(&meta, tooling)
}

func (b *Broker) injectTheiaRemotePlugin(meta model.PluginMeta, archiveFolder string, image string, pj *PackageJSON) error {
	pluginFolderPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s", meta.ID, meta.Version))
	b.PrintDebug("Copying Theia remote plugin '%s:%s' from '%s' to '%s'", meta.ID, meta.Version, archiveFolder, pluginFolderPath)
	err := b.ioUtil.CopyResource(archiveFolder, pluginFolderPath)
	if err != nil {
		return err
	}
	tooling := &model.ToolingConf{
		Containers: []model.Container{*b.containerConfig(image)},
	}
	b.addPortToTooling(tooling, pj)
	return b.storage.AddPlugin(&meta, tooling)
}

func (b *Broker) containerConfig(image string) *model.Container {
	c := model.Container{
		Name:  "theiapluginsidecar" + b.getRndNumberAsString(),
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

// addPortToTooling adds to tooling everything needed to start Theia remote plugin:
// - Random port to the container (one and only)
// - Endpoint matching the port
// - Environment variable THEIA_PLUGIN_ENDPOINT_PORT to the container with the port as value
// - Environment variable that start from THEIA_PLUGIN_REMOTE_ENDPOINT_ and ends with
// plugin publisher and plugin name taken from packageJson and replacing all
// chars matching [^a-z_0-9]+ with a dash character
func (b *Broker) addPortToTooling(toolingConf *model.ToolingConf, pj *PackageJSON) {
	port := b.getRndPort()
	sPort := strconv.Itoa(port)
	endpointName := "port" + sPort
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

func (b *Broker) getRndNumberAsString() string {
	port := b.getRndPort() // CHANge name to something meaningful and/OR random
	return strconv.Itoa(port)
}

func (b *Broker) getRndPort() int {
	return 4000 + rand.Intn(6000)
}
