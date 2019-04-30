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

package unified

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	jsonrpc "github.com/eclipse/che-go-jsonrpc"
	broker "github.com/eclipse/che-plugin-broker/brokers/che-plugin-broker"
	"github.com/eclipse/che-plugin-broker/brokers/theia"
	"github.com/eclipse/che-plugin-broker/brokers/vscode"
	"github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
	"github.com/eclipse/che-plugin-broker/utils"
	"gopkg.in/yaml.v2"
)

const ChePluginType = "che plugin"
const EditorPluginType = "che editor"
const TheiaPluginType = "theia plugin"
const VscodePluginType = "vs code extension"

// RegistryURLFormat specifies the format string for registry urls
// when downloading metas
const RegistryURLFormat = "%s/%s/meta.yaml"

// Broker is used to process Che plugins
type Broker struct {
	common.Broker
	Storage *storage.Storage
	utils   utils.IoUtil

	theiaBroker  common.BrokerImpl
	vscodeBroker common.BrokerImpl
	cheBroker    common.BrokerImpl
}

// NewBroker creates Che broker instance
func NewBroker() *Broker {
	commonBroker := common.NewBroker()
	ioUtils := utils.New()
	storageObj := storage.New()
	httpClient := &http.Client{}
	rand := common.NewRand()

	cheBroker := broker.NewBrokerWithParams(commonBroker, ioUtils, storageObj)
	theiaBroker := theia.NewBrokerWithParams(commonBroker, ioUtils, storageObj, rand)
	vscodeBroker := vscode.NewBrokerWithParams(commonBroker, ioUtils, storageObj, rand, httpClient)
	return &Broker{
		Broker:  commonBroker,
		Storage: storageObj,
		utils:   ioUtils,

		theiaBroker:  theiaBroker,
		vscodeBroker: vscodeBroker,
		cheBroker:    cheBroker,
	}
}

// DownloadMetasAndStart downloads metas from plugin registry for specified
// pluginFQNs and then executes plugins metas processing and sending data to Che master
func (b *Broker) Start(pluginFQNs []model.PluginFQN, defaultRegistry string) error {
	pluginMetas, err := b.getPluginMetas(pluginFQNs, defaultRegistry)
	if err != nil {
		message := fmt.Sprintf("Failed to download plugin meta: %s", err)
		b.PubFailed(message)
		b.PubLog(message)
		return errors.New(message)
	}
	defer b.CloseConsumers()
	b.PubStarted()
	b.PrintInfo("Unified Che Plugin Broker")
	b.PrintPlan(pluginMetas)

	err = b.ProcessPlugins(pluginMetas)
	if err != nil {
		b.PubFailed(err.Error())
		b.PubLog(err.Error())
		return err
	}

	result, err := b.serializeTooling()
	if err != nil {
		b.PubFailed(err.Error())
		b.PubLog(err.Error())
		return err
	}

	b.PrintInfo("All plugins have been successfully processed")
	b.PrintDebug(result)
	b.PubDone(result)
	return nil
}

// PushEvents sets given tunnel as consumer of broker events.
func (b *Broker) PushEvents(tun *jsonrpc.Tunnel) {
	b.Broker.PushEvents(tun, model.BrokerStatusEventType, model.BrokerResultEventType, model.BrokerLogEventType)
}

// ProcessPlugins processes metas of different plugin types and passes metas of each particular type
// to the appropriate plugin broker
func (b *Broker) ProcessPlugins(metas []model.PluginMeta) error {
	cheMetas, theiaMetas, vscodeMetas, err := sortMetas(metas)
	if err != nil {
		return err
	}

	b.PrintInfo("Starting Che common plugins processing")
	for _, meta := range cheMetas {
		err := b.cheBroker.ProcessPlugin(meta)
		if err != nil {
			return err
		}
	}
	b.PrintInfo("Starting Theia plugins processing")
	for _, meta := range theiaMetas {
		err := b.theiaBroker.ProcessPlugin(meta)
		if err != nil {
			return err
		}
	}
	b.PrintInfo("Starting VS Code plugins processing")
	for _, meta := range vscodeMetas {
		err := b.vscodeBroker.ProcessPlugin(meta)
		if err != nil {
			return err
		}
	}

	return nil
}

// getPluginMetas downloads the metadata for each plugin in plugins. If specified,
// defaultRegistry is used as the registry for plugins that do not specify their registry.
// If defaultRegistry is empty, and any plugin does not specify a registry, an error is returned.
func (b *Broker) getPluginMetas(plugins []model.PluginFQN, defaultRegistry string) ([]model.PluginMeta, error) {
	metas := make([]model.PluginMeta, 0, len(plugins))
	for _, plugin := range plugins {
		log.Printf("Fetching plugin meta.yaml for %s", plugin.ID)
		registry, err := getRegistryURL(plugin, defaultRegistry)
		if err != nil {
			return nil, err
		}
		pluginURL := fmt.Sprintf(RegistryURLFormat, registry, plugin.ID)
		pluginRaw, err := b.utils.Fetch(pluginURL)
		if err != nil {
			if httpErr, ok := err.(*utils.HTTPError); ok {
				return nil, fmt.Errorf(
					"failed to fetch plugin meta.yaml for plugin '%s' from registry '%s': %s. Response body: %s",
					plugin.ID, registry, httpErr, httpErr.Body)
			} else {
				return nil, fmt.Errorf(
					"failed to fetch plugin meta.yaml for plugin '%s' from registry '%s': %s",
					plugin.ID, registry, err)
			}
		}

		var pluginMeta model.PluginMeta
		if err := yaml.Unmarshal(pluginRaw, &pluginMeta); err != nil {
			return nil, fmt.Errorf(
				"failed to unmarshal downloaded meta.yaml for plugin '%s': %s", plugin.ID, err)
		}
		// Ensure ID field is set since it is used all over the place in broker
		if pluginMeta.ID == "" {
			pluginMeta.ID = plugin.ID
		}
		metas = append(metas, pluginMeta)
	}
	return metas, nil
}

func (b *Broker) serializeTooling() (string, error) {
	plugins, err := b.Storage.Plugins()
	if err != nil {
		return "", err
	}
	pluginsBytes, err := json.Marshal(plugins)
	if err != nil {
		return "", err
	}

	return string(pluginsBytes), nil
}

func sortMetas(metas []model.PluginMeta) (che []model.PluginMeta, theia []model.PluginMeta, vscode []model.PluginMeta, err error) {
	vscodeMetas := make([]model.PluginMeta, 0)
	theiaMetas := make([]model.PluginMeta, 0)
	cheBrokerMetas := make([]model.PluginMeta, 0)
	for _, meta := range metas {
		switch strings.ToLower(meta.Type) {
		case ChePluginType:
			fallthrough
		case EditorPluginType:
			cheBrokerMetas = append(cheBrokerMetas, meta)
		case VscodePluginType:
			vscodeMetas = append(vscodeMetas, meta)
		case TheiaPluginType:
			theiaMetas = append(theiaMetas, meta)
		case "":
			return nil, nil, nil, fmt.Errorf("Type field is missing in meta information of plugin '%s'", meta.ID)
		default:
			return nil, nil, nil, fmt.Errorf("Type '%s' of plugin '%s' is unsupported", meta.Type, meta.ID)
		}
	}

	return cheBrokerMetas, theiaMetas, vscodeMetas, nil
}

func getRegistryURL(plugin model.PluginFQN, defaultRegistry string) (string, error) {
	var registry string
	if plugin.Registry != "" {
		registry = strings.TrimSuffix(plugin.Registry, "/")
	} else {
		if defaultRegistry == "" {
			return "", fmt.Errorf("plugin '%s' does not specify registry and no default is provided", plugin.ID)
		}
		registry = strings.TrimSuffix(defaultRegistry, "/") + "/plugins"
	}
	return registry, nil
}
