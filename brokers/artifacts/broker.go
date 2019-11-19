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

package artifacts

import (
	"fmt"
	"path/filepath"

	jsonrpc "github.com/eclipse/che-go-jsonrpc"
	"github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/utils"
)

const errorNoExtFieldsTemplate = "Field 'extensions' is not found in the description of the plugin '%s'"

// Broker is used to process Che plugins
type Broker struct {
	common.Broker
	ioUtils utils.IoUtil
	rand    common.Random
}

// NewBroker creates Che broker instance
func NewBroker(localhostSidecar bool) *Broker {
	return &Broker{
		Broker:  common.NewBroker(),
		ioUtils: utils.New(),
		rand:    common.NewRand(),
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

// Start downloads metas from plugin registry for specified
// pluginFQNs and then executes plugins metas processing and sending data to Che master
func (b *Broker) Start(pluginFQNs []model.PluginFQN, defaultRegistry string) error {
	defer b.CloseConsumers()
	b.PubStarted()
	b.PrintInfo("Starting plugin artifacts broker")

	b.cleanupPluginsDirectory()
	pluginMetas, err := utils.GetPluginMetas(pluginFQNs, defaultRegistry, b.ioUtils)
	if err != nil {
		return b.fail(fmt.Errorf("Failed to download plugin meta: %s", err))
	}
	b.PrintInfo("Downloading plugin extensions")

	err = utils.ResolveRelativeExtensionPaths(pluginMetas, defaultRegistry)
	if err != nil {
		return b.fail(err)
	}

	for _, meta := range pluginMetas {
		err = b.ProcessPlugin(meta)
		if err != nil {
			return b.fail(err)
		}
	}

	b.PrintInfo("All plugin artifacts have been successfully downloaded")
	b.PubDone("")

	return nil
}

func (b *Broker) cleanupPluginsDirectory() {
	b.PrintInfo("Cleaning /plugins dir")
	files, err := b.ioUtils.GetFilesByGlob(filepath.Join("/plugins", "*"))
	if err != nil {
		// Send log about clearing failure but proceed.
		// We might want to change this behavior later
		b.PrintInfo("WARN: failed to clear /plugins directory. Error: %s", err)
		return
	}

	for _, file := range files {
		err = b.ioUtils.RemoveAll(file)
		if err != nil {
			b.PrintInfo("WARN: failed to remove '%s'. Error: %s", file, err)
		}
	}
}

// ProcessPlugin processes metas of different plugin types and passes metas of each particular type
// to the appropriate plugin broker
func (b *Broker) ProcessPlugin(meta model.PluginMeta) error {
	if !utils.IsTheiaOrVscodePlugin(meta) {
		return nil
	}

	URLs, err := getUrls(meta)
	if err != nil {
		return err
	}

	workDir, err := b.ioUtils.TempDir("", "vscode-extension-broker")
	if err != nil {
		return err
	}

	archivesPaths, err := b.downloadArchives(URLs, meta, workDir)
	if err != nil {
		return err
	}

	err = b.injectPlugin(meta, archivesPaths)
	return err
}

func (b *Broker) downloadArchives(URLs []string, meta model.PluginMeta, workDir string) ([]string, error) {
	paths := make([]string, 0)
	for i, URL := range URLs {
		archivePath := b.ioUtils.ResolveDestPathFromURL(URL, workDir)
		b.PrintDebug("Downloading VS Code extension archive '%s' for plugin '%s' to '%s'", URL, meta.ID, archivePath)
		b.PrintInfo("Downloading VS Code extension %d/%d for plugin '%s'", i+1, len(URLs), meta.ID)
		archivePath, err := b.ioUtils.Download(URL, archivePath, true)
		paths = append(paths, archivePath)
		if err != nil {
			return nil, fmt.Errorf("failed to download plugin from %s: %s", URL, err)
		}
	}
	return paths, nil
}

func (b *Broker) injectPlugin(meta model.PluginMeta, archivesPaths []string) error {
	for _, path := range archivesPaths {
		pluginPath := "/plugins"
		if len(meta.Spec.Containers) > 0 {
			// Plugin is remote
			pluginUniqueName := utils.GetPluginUniqueName(meta)
			pluginPath = filepath.Join(pluginPath, "sidecars", pluginUniqueName)
			err := b.ioUtils.MkDir(pluginPath)
			if err != nil {
				return err
			}
		}
		pluginArchiveName := b.generatePluginArchiveName(meta, path)
		pluginArchivePath := filepath.Join(pluginPath, pluginArchiveName)
		err := b.ioUtils.CopyFile(path, pluginArchivePath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Broker) generatePluginArchiveName(plugin model.PluginMeta, archivePath string) string {
	archiveName := filepath.Base(archivePath)
	return fmt.Sprintf("%s.%s.%s.%s.%s", plugin.Publisher, plugin.Name, plugin.Version, b.rand.String(10), archiveName)
}

func getUrls(meta model.PluginMeta) ([]string, error) {
	URLs := make([]string, 0)
	if len(meta.Spec.Extensions) == 0 {
		return nil, fmt.Errorf(errorNoExtFieldsTemplate, meta.ID)
	}
	URLs = append(URLs, meta.Spec.Extensions...)
	return URLs, nil
}
