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

	"bytes"
	jsonrpc "github.com/eclipse/che-go-jsonrpc"
	"github.com/eclipse/che-go-jsonrpc/event"
	"github.com/eclipse/che-plugin-broker/cfg"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
	yaml "gopkg.in/yaml.v2"
	"time"
)

var (
	bus = event.NewBus()
)

// Start executes plugins metas processing and sends data to Che master
func Start(metas []model.PluginMeta) {
	if ok, status := storage.SetStatus(model.StatusStarted); !ok {
		m := fmt.Sprintf("Starting broker in state '%s' is not allowed", status)
		pubFailed(m)
		printFatal(m)
	}
	pubStarted()
	printInfo("Started Plugin Broker")

	// Clear any existing plugins from dir
	log.Println("Cleaning /plugins dir")
	err := clearDir("/plugins")
	if err != nil {
		log.Printf("WARN: failed to clear /plugins directory: %s", err)
	}

	printPlan(metas)

	printInfo("Starting plugins processing")
	for _, meta := range metas {
		err := processPlugin(meta)
		if err != nil {
			pubFailed(err.Error())
			printFatal(err.Error())
		}
	}

	if ok, status := storage.SetStatus(model.StatusDone); !ok {
		err := fmt.Sprintf("Setting '%s' broker status failed. Broker has '%s' state", model.StatusDone, status)
		pubFailed(err)
		printFatal(err)
	}

	tooling, err := storage.Tooling()
	if err != nil {
		pubFailed(err.Error())
		printFatal(err.Error())
	}
	bytes, err := json.Marshal(tooling)
	if err != nil {
		pubFailed(err.Error())
		printFatal(err.Error())
	}

	printInfo("All plugins have been successfully processed")
	pubDone(string(bytes))
	closeConsumers()
}

func closeConsumers() {
	for _, candidates := range bus.Clear() {
		for _, candidate := range candidates {
			if broadcaster, ok := candidate.(*tunnelBroadcaster); ok {
				broadcaster.Close()
			}
		}
	}
}

func (tb *tunnelBroadcaster) Close() { tb.tunnel.Close() }

func pubStarted() {
	bus.Pub(&model.StartedEvent{
		Status:    model.StatusStarted,
		RuntimeID: cfg.RuntimeID,
	})
}

func pubFailed(err string) {
	bus.Pub(&model.ErrorEvent{
		Status:    model.StatusFailed,
		Error:     err,
		RuntimeID: cfg.RuntimeID,
	})
}

func pubDone(tooling string) {
	bus.Pub(&model.SuccessEvent{
		Status:    model.StatusDone,
		RuntimeID: cfg.RuntimeID,
		Tooling:   tooling,
	})
}

// PushEvents sets given tunnel as consumer of broker events.
func PushEvents(tun *jsonrpc.Tunnel) {
	bus.SubAny(&tunnelBroadcaster{tunnel: tun}, model.BrokerStatusEventType, model.BrokerResultEventType, model.BrokerLogEventType)
}

type tunnelBroadcaster struct {
	tunnel *jsonrpc.Tunnel
}

func (tb *tunnelBroadcaster) Accept(e event.E) {
	if err := tb.tunnel.Notify(e.Type(), e); err != nil {
		log.Fatalf("Trying to send event of type '%s' to closed tunnel '%s'", e.Type(), tb.tunnel.ID())
	}
}

func processPlugin(meta model.PluginMeta) error {
	printDebug("Stared processing plugin '%s:%s'", meta.ID, meta.Version)
	url := meta.URL

	workDir, err := ioutil.TempDir("", "che-plugin-broker")
	if err != nil {
		return err
	}

	archivePath := filepath.Join(workDir, "testArchive.tar.gz")
	pluginPath := filepath.Join(workDir, "testArchive")

	// Download an archive
	printDebug("Downloading archive '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, archivePath)
	err = download(url, archivePath)
	if err != nil {
		return err
	}

	// Untar it
	printDebug("Untarring archive '%s' for plugin '%s:%s' to '%s'", url, meta.ID, meta.Version, archivePath)
	err = untar(archivePath, pluginPath)
	if err != nil {
		return err
	}

	printDebug("Resolving Che plugins for '%s:%s'", meta.ID, meta.Version)
	err = resolveToolingConfig(pluginPath)
	if err != nil {
		return err
	}

	printDebug("Copying dependencies for '%s:%s'", meta.ID, meta.Version)
	return copyDependencies(pluginPath)
}

func resolveToolingConfig(workDir string) error {
	toolingConfPath := filepath.Join(workDir, "che-plugin.yaml")
	f, err := ioutil.ReadFile(toolingConfPath)
	if err != nil {
		return err
	}

	tooling := &model.ToolingConf{}
	if err := yaml.Unmarshal(f, tooling); err != nil {
		return err
	}

	return storage.AddTooling(tooling)
}

func copyDependencies(workDir string) error {
	depsConfPath := filepath.Join(workDir, "che-dependency.yaml")
	if _, err := os.Stat(depsConfPath); os.IsNotExist(err) {
		return nil
	}

	f, err := ioutil.ReadFile(depsConfPath)
	if err != nil {
		return err
	}

	deps := &model.CheDependencies{}
	if err := yaml.Unmarshal(f, deps); err != nil {
		return err
	}

	for _, dep := range deps.Plugins {
		switch {
		case dep.Location != "" && dep.URL != "":
			m := fmt.Sprintf("Plugin dependency '%s:%s' contains both 'location' and 'url' fields while just one should be present", dep.ID, dep.Version)
			return errors.New(m)
		case dep.Location != "":
			fileDest := resolveDestPath(dep.Location, "/plugins")
			fileSrc := filepath.Join(workDir, dep.Location)
			printDebug("Copying file '%s' to '%s'", fileSrc, fileDest)
			if err = copyFile(fileSrc, fileDest); err != nil {
				return err
			}
		case dep.URL != "":
			fileDest := resolveDestPathFromURL(dep.URL, "/plugins")
			printDebug("Downloading file '%s' to '%s'", dep.URL, fileDest)
			if err = download(dep.URL, fileDest); err != nil {
				return err
			}
		default:
			m := fmt.Sprintf("Plugin dependency '%s:%s' contains neither 'location' nor 'url' field", dep.ID, dep.Version)
			return errors.New(m)
		}
	}

	return nil
}

func printDebug(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func printInfo(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	bus.Pub(&model.PluginBrokerLogEvent{
		RuntimeID: cfg.RuntimeID,
		Text:      message,
		Time:      time.Now(),
	})
}

func printFatal(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	log.Fatal(message)
}

func printPlan(metas []model.PluginMeta) {
	var buffer bytes.Buffer

	buffer.WriteString("List of plugins and editors to install\n")
	for _, plugin := range metas {
		buffer.WriteString(fmt.Sprintf("- %s:%s - %s\n", plugin.ID, plugin.Version, plugin.Description))
	}

	printInfo(buffer.String())
}
