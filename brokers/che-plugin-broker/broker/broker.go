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
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/eclipse/che-go-jsonrpc"
	"github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/files"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
)

const pluginFileName = "che-plugin.yaml"
const depFileName = "che-dependency.yaml"
const depFileBothLocationAndURLError = "Plugin dependency '%s:%s' contains both 'location' and 'url' fields while just one should be present"
const depFileNoLocationURLError = "Plugin dependency '%s:%s' contains neither 'location' nor 'url' field"

type PluginLinkType int
const (
	Archive PluginLinkType = iota + 1
	Yaml
)

// ChePluginBroker is used to process Che plugins
type ChePluginBroker struct {
	common.Broker
	ioUtil  files.IoUtil
	storage *storage.Storage
}

// NewBroker creates Che plugin broker instance
func NewBroker() *ChePluginBroker {
	return &ChePluginBroker{
		common.NewBroker(),
		files.New(),
		storage.New(),
	}
}

// Start executes plugins metas processing and sends data to Che master
func (cheBroker *ChePluginBroker) Start(metas []model.PluginMeta) {
	if ok, status := cheBroker.storage.SetStatus(model.StatusStarted); !ok {
		m := fmt.Sprintf("Starting broker in state '%s' is not allowed", status)
		cheBroker.PubFailed(m)
		cheBroker.PrintFatal(m)
	}
	cheBroker.PubStarted()
	cheBroker.PrintInfo("Started Plugin Broker")

	// Clear any existing plugins from dir
	log.Println("Cleaning /plugins dir")
	err := cheBroker.ioUtil.ClearDir("/plugins")
	if err != nil {
		log.Printf("WARN: failed to clear /plugins directory: %s", err)
	}

	cheBroker.PrintPlan(metas)

	cheBroker.PrintInfo("Starting plugins processing")
	for _, meta := range metas {
		err := cheBroker.processPlugin(meta)
		if err != nil {
			cheBroker.PubFailed(err.Error())
			cheBroker.PrintFatal(err.Error())
		}
	}

	if ok, status := cheBroker.storage.SetStatus(model.StatusDone); !ok {
		err := fmt.Sprintf("Setting '%s' broker status failed. Broker has '%s' state", model.StatusDone, status)
		cheBroker.PubFailed(err)
		cheBroker.PrintFatal(err)
	}

	plugins, err := cheBroker.storage.Plugins()
	if err != nil {
		cheBroker.PubFailed(err.Error())
		cheBroker.PrintFatal(err.Error())
	}
	pluginsBytes, err := json.Marshal(plugins)
	if err != nil {
		cheBroker.PubFailed(err.Error())
		cheBroker.PrintFatal(err.Error())
	}

	cheBroker.PrintInfo("All plugins have been successfully processed")
	cheBroker.PubDone(string(pluginsBytes))
	cheBroker.CloseConsumers()
}

// PushEvents sets given tunnel as consumer of broker events.
func (cheBroker *ChePluginBroker) PushEvents(tun *jsonrpc.Tunnel) {
	cheBroker.Broker.PushEvents(tun, model.BrokerStatusEventType, model.BrokerResultEventType, model.BrokerLogEventType)
}

func (cheBroker *ChePluginBroker) processPlugin(meta model.PluginMeta) error {
	cheBroker.PrintDebug("Stared processing plugin '%s:%s'", meta.ID, meta.Version)
	url := meta.URL

	switch getTypeOfURL(url) {
	case Archive:
		return cheBroker.processArchive(&meta, url)
	case Yaml:
		return cheBroker.processYAML(&meta, url)
	}
	return nil
}
func (cheBroker *ChePluginBroker) processYAML(meta *model.PluginMeta, url string) error {
	workDir, err := cheBroker.ioUtil.TempDir("", "che-plugin-broker")
	if err != nil {
		return err
	}
	var pluginPath string

	chePluginYamlPath := filepath.Join(workDir, pluginFileName)
	cheBroker.PrintDebug("Downloading plugin definition '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, chePluginYamlPath)
	err = cheBroker.ioUtil.Download(url, chePluginYamlPath)
	if err != nil {
		return err
	}
	pluginPath = workDir
	cheBroker.PrintDebug("Resolving '%s:%s'", meta.ID, meta.Version)
	err = cheBroker.resolveToolingConfig(meta, pluginPath)
	if err != nil {
		return err
	}
	return nil
}

func (cheBroker *ChePluginBroker) processArchive(meta *model.PluginMeta, url string) error {
	workDir, err := cheBroker.ioUtil.TempDir("", "che-plugin-broker")
	if err != nil {
		return err
	}
	var pluginPath string

	archivePath := filepath.Join(workDir, "pluginArchive.tar.gz")
	pluginPath = filepath.Join(workDir, "plugin")

	// Download an archive
	cheBroker.PrintDebug("Downloading archive '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, archivePath)
	err = cheBroker.ioUtil.Download(url, archivePath)
	if err != nil {
		return err
	}

	// Untar it
	cheBroker.PrintDebug("Untarring archive '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, archivePath)
	err = cheBroker.ioUtil.Untar(archivePath, pluginPath)
	if err != nil {
		return err
	}
	cheBroker.PrintDebug("Resolving '%s:%s'", meta.ID, meta.Version)
	err = cheBroker.resolveToolingConfig(meta, pluginPath)
	if err != nil {
		return err
	}
	cheBroker.PrintDebug("Copying dependencies for '%s:%s'", meta.ID, meta.Version)
	return cheBroker.copyDependencies(pluginPath)
}

func getTypeOfURL(url string) PluginLinkType {

	if strings.HasSuffix(url, pluginFileName) {
		return Yaml
	}
	return Archive
}

func (cheBroker *ChePluginBroker) resolveToolingConfig(meta *model.PluginMeta, workDir string) error {
	toolingConfPath := filepath.Join(workDir, pluginFileName)
	f, err := ioutil.ReadFile(toolingConfPath)
	if err != nil {
		return err
	}

	tooling := &model.ToolingConf{}
	if err := yaml.Unmarshal(f, tooling); err != nil {
		return err
	}

	return cheBroker.storage.AddPlugin(meta, tooling)
}

func (cheBroker *ChePluginBroker) copyDependencies(workDir string) error {
	deps, err := cheBroker.parseDepsFile(workDir)
	if err != nil || deps == nil {
		return err
	}

	for _, dep := range deps.Plugins {
		switch {
		case dep.Location != "" && dep.URL != "":
			m := fmt.Sprintf(depFileBothLocationAndURLError, dep.ID, dep.Version)
			return errors.New(m)
		case dep.Location != "":
			fileDest := cheBroker.ioUtil.ResolveDestPath(dep.Location, "/plugins")
			fileSrc := filepath.Join(workDir, dep.Location)
			cheBroker.PrintDebug("Copying resource '%s' to '%s'", fileSrc, fileDest)
			if err = cheBroker.ioUtil.CopyResource(fileSrc, fileDest); err != nil {
				return err
			}
		case dep.URL != "":
			fileDest := cheBroker.ioUtil.ResolveDestPathFromURL(dep.URL, "/plugins")
			cheBroker.PrintDebug("Downloading file '%s' to '%s'", dep.URL, fileDest)
			if err = cheBroker.ioUtil.Download(dep.URL, fileDest); err != nil {
				return err
			}
		default:
			m := fmt.Sprintf(depFileNoLocationURLError, dep.ID, dep.Version)
			return errors.New(m)
		}
	}

	return nil
}

func (cheBroker *ChePluginBroker) parseDepsFile(workDir string) (*model.CheDependencies, error) {
	depsConfPath := filepath.Join(workDir, depFileName)
	if _, err := os.Stat(depsConfPath); os.IsNotExist(err) {
		return nil, nil
	}

	f, err := ioutil.ReadFile(depsConfPath)
	if err != nil {
		return nil, err
	}

	deps := &model.CheDependencies{}
	if err := yaml.Unmarshal(f, deps); err != nil {
		return nil, err
	}
	return deps, nil
}
