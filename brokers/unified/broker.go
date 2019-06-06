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
	"github.com/eclipse/che-plugin-broker/brokers/unified/vscode"
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
	Storage storage.Storage
	utils   utils.IoUtil

	vscodeBroker common.BrokerImpl
}

// NewBrokerWithParams creates Che broker instance with parameters
func NewBrokerWithParams(
	commonBroker common.Broker,
	ioUtil utils.IoUtil,
	storage storage.Storage,
	rand common.Random,
	httpClient *http.Client,
	localhostSidecar bool) *Broker {
	vscodeBroker := vscode.NewBrokerWithParams(commonBroker, ioUtil, storage, rand, httpClient, localhostSidecar)
	return &Broker{
		Broker:       commonBroker,
		Storage:      storage,
		utils:        ioUtil,
		vscodeBroker: vscodeBroker,
	}
}

// NewBroker creates Che broker instance
func NewBroker(localhostSidecar bool) *Broker {
	return NewBrokerWithParams(common.NewBroker(), utils.New(), storage.New(), common.NewRand(), &http.Client{}, localhostSidecar)
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
	err := validateMetas(metas)
	if err != nil {
		return err
	}

	cheMetas, vscodeMetas, err := sortMetas(metas)
	if err != nil {
		return err
	}

	b.PrintInfo("Starting Che plugins and editor processing")
	for _, meta := range cheMetas {
		plugin := ConvertMetaToPlugin(meta)
		err = b.Storage.AddPlugin(plugin)
		if err != nil {
			return err
		}
	}

	b.PrintInfo("Starting VS Code and Theia plugins processing")
	for _, meta := range vscodeMetas {
		err := b.vscodeBroker.ProcessPlugin(meta)
		if err != nil {
			return err
		}
	}

	return nil
}

func ValidateMeta(meta model.PluginMeta) error {
	switch meta.APIVersion {
	case "":
		return fmt.Errorf("Plugin '%s' is invalid. Field 'apiVersion' must be present", meta.ID)
	case "v2":
		// validate here something
	default:
		return fmt.Errorf("Plugin '%s' is invalid. Field 'apiVersion' contains invalid version '%s'", meta.ID, meta.APIVersion)
	}

	switch strings.ToLower(meta.Type) {
	case ChePluginType:
		fallthrough
	case EditorPluginType:
		if len(meta.Spec.Extensions) != 0 {
			return fmt.Errorf("Plugin '%s' is invalid. Field 'spec.extensions' is not allowed in plugin of type '%s'", meta.ID, meta.Type)
		}
		if len(meta.Spec.Containers) == 0 {
			return fmt.Errorf("Plugin '%s' is invalid. Field 'spec.containers' must not be empty", meta.ID)
		}
	case TheiaPluginType:
		fallthrough
	case VscodePluginType:
		if len(meta.Spec.Extensions) == 0 {
			return fmt.Errorf("Plugin '%s' is invalid. Field 'spec.extensions' must not be empty", meta.ID)
		}
		if len(meta.Spec.Containers) > 1 {
			return fmt.Errorf("Plugin '%s' is invalid. Containers list 'spec.containers' must not contain more than 1 container, but '%d' found", meta.ID, len(meta.Spec.Containers))
		}
		if len(meta.Spec.Endpoints) != 0 {
			return fmt.Errorf("Plugin '%s' is invalid. Setting endpoints at 'spec.endpoints' is not allowed in plugins of type '%s'", meta.ID, meta.Type)
		}
	}
	return nil
}

func validateMetas(metas []model.PluginMeta) error {
	for _, meta := range metas {
		if err := ValidateMeta(meta); err != nil {
			return err
		}
	}
	return nil
}

// GetPluginMeta downloads the metadata for a plugin. If specified,
// defaultRegistry is used as the registry when plugin does not specify its registry.
// If defaultRegistry is empty, and plugin does not specify a registry, an error is returned.
func (b *Broker) GetPluginMeta(plugin model.PluginFQN, defaultRegistry string) (*model.PluginMeta, error) {
	var pluginURL string
	if plugin.Reference != "" {
		pluginURL = plugin.Reference
	} else {
		registry, err := getRegistryURL(plugin, defaultRegistry)
		if err != nil {
			return nil, err
		}
		pluginURL = fmt.Sprintf(RegistryURLFormat, registry, plugin.ID)
		log.Printf("Fetching plugin meta.yaml from %s", pluginURL)
	}
	pluginRaw, err := b.utils.Fetch(pluginURL)
	if err != nil {
		if httpErr, ok := err.(*utils.HTTPError); ok {
			return nil, fmt.Errorf(
				"failed to fetch plugin meta.yaml from URL '%s': %s. Response body: %s",
				pluginURL, httpErr, httpErr.Body)
		} else {
			return nil, fmt.Errorf(
				"failed to fetch plugin meta.yaml from URL '%s': %s",
				pluginURL, err)
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
	return &pluginMeta, nil
}

// getPluginMetas downloads the metadata for each plugin in plugins. If specified,
// defaultRegistry is used as the registry for plugins that do not specify their registry.
// If defaultRegistry is empty, and any plugin does not specify a registry, an error is returned.
func (b *Broker) getPluginMetas(plugins []model.PluginFQN, defaultRegistry string) ([]model.PluginMeta, error) {
	metas := make([]model.PluginMeta, 0, len(plugins))
	for _, plugin := range plugins {
		pluginMeta, err := b.GetPluginMeta(plugin, defaultRegistry)
		if err != nil {
			return nil, err
		}
		metas = append(metas, *pluginMeta)
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

func sortMetas(metas []model.PluginMeta) (che []model.PluginMeta, vscode []model.PluginMeta, err error) {
	vscodeMetas := make([]model.PluginMeta, 0)
	cheBrokerMetas := make([]model.PluginMeta, 0)
	for _, meta := range metas {
		switch strings.ToLower(meta.Type) {
		case ChePluginType:
			fallthrough
		case EditorPluginType:
			cheBrokerMetas = append(cheBrokerMetas, meta)
		case TheiaPluginType:
			fallthrough
		case VscodePluginType:
			vscodeMetas = append(vscodeMetas, meta)
		case "":
			return nil, nil, fmt.Errorf("Type field is missing in meta information of plugin '%s'", meta.ID)
		default:
			return nil, nil, fmt.Errorf("Type '%s' of plugin '%s' is unsupported", meta.Type, meta.ID)
		}
	}

	return cheBrokerMetas, vscodeMetas, nil
}

func getRegistryURL(plugin model.PluginFQN, defaultRegistry string) (string, error) {
	var registry string
	if plugin.Registry != "" {
		registry = strings.TrimSuffix(plugin.Registry, "/") + "/plugins"
	} else {
		if defaultRegistry == "" {
			return "", fmt.Errorf("plugin '%s' does not specify registry and no default is provided", plugin.ID)
		}
		registry = strings.TrimSuffix(defaultRegistry, "/") + "/plugins"
	}
	return registry, nil
}

func ConvertMetaToPlugin(meta model.PluginMeta) model.ChePlugin {
	return model.ChePlugin{
		ID:           meta.ID,
		Name:         meta.Name,
		Publisher:    meta.Publisher,
		Version:      meta.Version,
		Containers:   meta.Spec.Containers,
		Endpoints:    meta.Spec.Endpoints,
		WorkspaceEnv: meta.Spec.WorkspaceEnv,
	}
}
