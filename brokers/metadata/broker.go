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

package metadata

import (
	"fmt"
	"regexp"

	jsonrpc "github.com/eclipse/che-go-jsonrpc"
	"github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
	"github.com/eclipse/che-plugin-broker/utils"
)

// RegistryURLFormat specifies the format string for registry urls
// when downloading metas
const RegistryURLFormat = "%s/%s/meta.yaml"

var re = regexp.MustCompile(`[^a-zA-Z_0-9]+`)

// Broker is used to process Che plugins
type Broker struct {
	common.Broker
	storage          storage.Storage
	ioUtils          utils.IoUtil
	rand             common.Random
	localhostSidecar bool
}

// NewBrokerWithParams creates Che broker instance with parameters
func NewBrokerWithParams(
	commonBroker common.Broker,
	ioUtil utils.IoUtil,
	storage storage.Storage,
	rand common.Random,
	localhostSidecar bool) *Broker {
	return &Broker{
		Broker:           commonBroker,
		storage:          storage,
		ioUtils:          ioUtil,
		rand:             rand,
		localhostSidecar: localhostSidecar,
	}
}

// NewBroker creates Che broker instance
func NewBroker(localhostSidecar bool) *Broker {
	return NewBrokerWithParams(common.NewBroker(), utils.New(), storage.New(), common.NewRand(), localhostSidecar)
}

func (b *Broker) fail(err error) error {
	b.PubFailed(err.Error())
	b.PubLog(err.Error())
	return err
}

// Start downloads metas from plugin registry for specified
// pluginFQNs and then executes plugins metas processing and sending data to Che master
func (b *Broker) Start(pluginFQNs []model.PluginFQN, defaultRegistry string) error {
	pluginMetas, err := utils.GetPluginMetas(pluginFQNs, defaultRegistry, b.ioUtils)
	if err != nil {
		return b.fail(fmt.Errorf("Failed to download plugin meta: %s", err))
	}
	defer b.CloseConsumers()
	b.PubStarted()
	b.PrintInfo("Metadata plugin broker")
	b.PrintPlan(pluginMetas)

	err = utils.ResolveRelativeExtensionPaths(pluginMetas, defaultRegistry)
	if err != nil {
		return b.fail(err)
	}

	err = b.ProcessPlugins(pluginMetas)
	if err != nil {
		return b.fail(err)
	}

	result, err := b.serializeTooling()
	if err != nil {
		return b.fail(err)
	}

	b.PrintInfo("All plugins have been successfully processed")
	b.PrintDebug(result)
	b.PubDone(result)
	return nil
}

// ProcessPlugins processes metas of different plugin types and passes metas of each particular type
// to the appropriate plugin broker
func (b *Broker) ProcessPlugins(metas []model.PluginMeta) error {
	err := utils.ValidateMetas(metas...)
	if err != nil {
		return err
	}

	for _, meta := range metas {
		plugin := ConvertMetaToPlugin(meta)

		if isTheiaOrVscodePlugin(meta) && len(meta.Spec.Containers) > 0 {
			AddPluginRunnerRequirements(plugin, b.rand, b.localhostSidecar)
		}

		err = b.storage.AddPlugin(plugin)
		if err != nil {
			return err
		}
	}

	return nil
}

// PushEvents sets given tunnel as consumer of broker events.
func (b *Broker) PushEvents(tun *jsonrpc.Tunnel) {
	b.Broker.PushEvents(tun, model.BrokerStatusEventType, model.BrokerResultEventType, model.BrokerLogEventType)
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
	}
}
