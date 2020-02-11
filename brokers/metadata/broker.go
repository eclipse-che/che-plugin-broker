//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package metadata

import (
	"encoding/json"
	"fmt"

	jsonrpc "github.com/eclipse/che-go-jsonrpc"
	"github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
	"github.com/eclipse/che-plugin-broker/utils"
)

// RegistryURLFormat specifies the format string for registry urls
// when downloading metas
const RegistryURLFormat = "%s/%s/meta.yaml"

// Broker is used to process Che plugins
type Broker struct {
	common.Broker
	Storage          storage.Storage
	ioUtils          utils.IoUtil
	rand             common.Random
	localhostSidecar bool
}

// NewBroker creates Che broker instance
func NewBroker(localhostSidecar bool) *Broker {
	return &Broker{
		Broker:           common.NewBroker(),
		Storage:          storage.New(),
		ioUtils:          utils.New(),
		rand:             common.NewRand(),
		localhostSidecar: localhostSidecar,
	}
}

func (b *Broker) fail(err error) error {
	b.PubFailed(err.Error())
	b.PubLog(err.Error())
	return err
}

// PushEvents sets given tunnel as consumer of broker events.
func (b *Broker) PushEvents(tun *jsonrpc.Tunnel) {
	b.Broker.PushEvents(tun, model.BrokerStatusEventType, model.BrokerResultEventType, model.BrokerLogEventType)
}

// Start the plugin brokering process for given plugin FQNs. Default registry is required
// only if not all plugins specify a registry.
func (b *Broker) Start(pluginFQNs []model.PluginFQN, defaultRegistry string) error {
	defer b.CloseConsumers()
	b.PubStarted()
	b.PrintInfo("Starting plugin metadata broker")

	pluginMetas, err := utils.GetPluginMetas(pluginFQNs, defaultRegistry, b.ioUtils)
	if err != nil {
		return b.fail(fmt.Errorf("Failed to download plugin meta: %s", err))
	}
	b.PrintPlan(pluginMetas)

	if collisions := utils.GetExtensionCollisions(pluginMetas); len(collisions) > 0 {
		collisionLog := []string{"WARNING: multiple instances of the same extension will be included in this workspace:"}
		collisionLog = append(collisionLog, utils.ConvertCollisionsToLog(collisions)...)
		collisionLog = append(collisionLog, "These plugins may not work as expected. If errors occur please try disabling all but one of the conflicting plugins.")
		b.PrintInfoBuffer(collisionLog)
	}

	// Process plugins into ChePlugins
	err = b.ProcessPlugins(pluginMetas)
	if err != nil {
		return b.fail(err)
	}

	// Serialize ChePlugins and return to Che server
	result, err := b.serializeTooling()
	if err != nil {
		return b.fail(err)
	}

	b.PrintInfo("All plugin metadata has been successfully processed")
	b.PrintDebug(result)
	b.PubDone(result)
	return nil
}

// ProcessPlugins converts a list of Plugin Metas into Che Plugins to be understood
// by the Che server, adding them to storage for later retrieval. Additionally,
// ProcessPlugins performs minimal validation. See also: ProcessPlugin
func (b *Broker) ProcessPlugins(metas []model.PluginMeta) error {
	err := utils.ValidateMetas(metas...)
	if err != nil {
		return err
	}

	remoteInjection, err := GetRuntimeInjection(metas)
	if err != nil {
		return fmt.Errorf("failed to get remote runtime injection: %s", err)
	}

	for _, meta := range metas {
		err := b.ProcessPlugin(meta, remoteInjection)
		if err != nil {
			return err
		}
	}
	return nil
}

// ProcessPlugin processes a single plugin, adding any necessary requirements for
// running the plugin in a Che workspace. Converts plugin meta to Che plugin, and adds
// it to storage for later retrieval. Parameter remoteInjection represents the environment
// variables and volumes potentially required by plugins for running the remote Theia
// runtime (see: GetRuntimeInjection)
func (b *Broker) ProcessPlugin(meta model.PluginMeta, remoteInjection *RemotePluginInjection) error {

	if utils.IsTheiaOrVscodePlugin(meta) && len(meta.Spec.Containers) > 0 {
		AddPluginRunnerRequirements(meta, b.rand, b.localhostSidecar)
		InjectRemoteRuntime(&meta, remoteInjection)
	}

	plugin := ConvertMetaToPlugin(meta)
	return b.Storage.AddPlugin(plugin)
}

// ConvertMetaToPlugin converts model.PluginMeta to model.ChePlugin, to allow the plugin configuration
// to be passed back to Che.
func ConvertMetaToPlugin(meta model.PluginMeta) model.ChePlugin {
	return model.ChePlugin{
		ID:             meta.ID,
		Name:           meta.Name,
		Publisher:      meta.Publisher,
		Version:        meta.Version,
		Containers:     meta.Spec.Containers,
		InitContainers: meta.Spec.InitContainers,
		Endpoints:      meta.Spec.Endpoints,
		WorkspaceEnv:   meta.Spec.WorkspaceEnv,
		Type:           meta.Type,
	}
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
