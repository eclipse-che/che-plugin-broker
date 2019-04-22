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

package theia

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	jsonrpc "github.com/eclipse/che-go-jsonrpc"
	"github.com/eclipse/che-plugin-broker/cfg"
	"github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
	"github.com/eclipse/che-plugin-broker/utils"
)

type brokerImpl struct {
	common.Broker
	ioUtil  utils.IoUtil
	storage *storage.Storage
	rand    common.Random
}

// NewBroker creates Che Theia plugin broker instance
func NewBroker() common.BrokerImpl {
	return &brokerImpl{
		Broker:  common.NewBroker(),
		ioUtil:  utils.New(),
		storage: storage.New(),
		rand:    common.NewRand(),
	}
}

// NewBrokerWithParams creates Che Theia plugin broker instance
func NewBrokerWithParams(broker common.Broker, ioUtil utils.IoUtil, storage *storage.Storage, rand common.Random) common.BrokerImpl {
	return &brokerImpl{
		Broker:  broker,
		ioUtil:  ioUtil,
		rand:    rand,
		storage: storage,
	}
}

// Start executes plugins metas processing and sends data to Che master
func (b *brokerImpl) Start(metas []model.PluginMeta) {
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
		err := b.ProcessPlugin(meta)
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
func (b *brokerImpl) PushEvents(tun *jsonrpc.Tunnel) {
	b.Broker.PushEvents(tun, model.BrokerStatusEventType, model.BrokerResultEventType, model.BrokerLogEventType)
}

func (b *brokerImpl) ProcessPlugin(meta model.PluginMeta) error {
	b.PrintDebug("Started processing plugin '%s'", meta.ID)
	url := meta.URL

	workDir, err := b.ioUtil.TempDir("", "theia-plugin-broker")
	if err != nil {
		return err
	}

	archivePath := filepath.Join(workDir, "pluginArchive")
	unpackedPath := filepath.Join(workDir, "plugin")

	// Download an archive
	b.PrintDebug("Downloading archive '%s' for plugin '%s' to '%s'", url, meta.ID, archivePath)
	err = b.ioUtil.Download(url, archivePath)
	if err != nil {
		return err
	}

	// Unzip it
	b.PrintDebug("Unzipping archive '%s' for plugin '%s' to '%s'", url, meta.ID, unpackedPath)
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
		if !cfg.OnlyApplyMetadataActions {
			err = b.injectTheiaFile(meta, archivePath)
			if err != nil {
				return err
			}
		}
		return b.storage.AddPlugin(&meta, &model.ToolingConf{})
	}
	// remote plugin
	return b.injectTheiaRemotePlugin(meta, unpackedPath, pluginImage, pj)
}

func (b *brokerImpl) getPackageJSON(pluginFolder string) (*PackageJSON, error) {
	packageJSONPath := filepath.Join(pluginFolder, "package.json")
	f, err := ioutil.ReadFile(packageJSONPath)
	if err != nil {
		return nil, err
	}
	pj := &PackageJSON{}
	err = json.Unmarshal(f, pj)
	return pj, err
}

func (b *brokerImpl) getPluginImage(pj *PackageJSON) (string, error) {
	if pj.Engines.CheRuntimeContainer != "" {
		return pj.Engines.CheRuntimeContainer, nil
	}
	return "", nil
}

func (b *brokerImpl) injectTheiaFile(meta model.PluginMeta, archivePath string) error {
	b.PrintDebug("Copying Theia plugin '%s'", meta.ID)
	pluginPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s.%s.theia", meta.Publisher, meta.Name, meta.Version))
	err := b.ioUtil.CopyFile(archivePath, pluginPath)
	if err != nil {
		return err
	}
	return nil
}

func (b *brokerImpl) injectTheiaRemotePlugin(meta model.PluginMeta, archiveFolder string, image string, pj *PackageJSON) error {
	if !cfg.OnlyApplyMetadataActions {
		pluginFolderPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s.%s", meta.Publisher, meta.Name, meta.Version))
		b.PrintDebug("Copying Theia remote plugin '%s' from '%s' to '%s'", meta.ID, archiveFolder, pluginFolderPath)
		err := b.ioUtil.CopyResource(archiveFolder, pluginFolderPath)
		if err != nil {
			return err
		}
	}
	tooling := GenerateSidecarTooling(image, pj.PackageJSON, b.rand)
	return b.storage.AddPlugin(&meta, tooling)
}
