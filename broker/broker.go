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

	jsonrpc "github.com/eclipse/che-go-jsonrpc"
	"github.com/eclipse/che-go-jsonrpc/event"
	"github.com/eclipse/che-plugin-broker/cfg"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
	yaml "gopkg.in/yaml.v2"
)

var (
	bus = event.NewBus()
)

// Start executes plugins metas processing and sends data to Che master
func Start(metas []model.PluginMeta) {
	if ok, status := storage.SetStatus(model.StatusStarted); !ok {
		m := fmt.Sprintf("Starting broker in state '%s' is not allowed", status)
		pubFailed(m)
		log.Fatal(m)
	}
	pubStarted()

	// Clear any existing plugins from dir
	log.Println("Cleaning /plugins dir")
	err := clearDir("/plugins")
	if err != nil {
		log.Printf("WARN: failed to clear /plugins directory: %s", err)
	}

	for _, meta := range metas {
		err := processPlugin(meta)
		if err != nil {
			pubFailed(err.Error())
			log.Fatal(err)
		}
	}

	if ok, status := storage.SetStatus(model.StatusDone); !ok {
		err := fmt.Sprintf("Setting '%s' broker status failed. Broker has '%s' state", model.StatusDone, status)
		pubFailed(err)
		log.Fatalf(err)
	}

	tooling, err := storage.Tooling()
	if err != nil {
		pubFailed(err.Error())
		log.Fatalf(err.Error())
	}
	bytes, err := json.Marshal(tooling)
	if err != nil {
		pubFailed(err.Error())
		log.Fatalf(err.Error())
	}
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

// PushStatuses sets given tunnel as consumer of broker events.
func PushStatuses(tun *jsonrpc.Tunnel) {
	bus.SubAny(&tunnelBroadcaster{tunnel: tun}, model.BrokerStatusEventType, model.BrokerResultEventType)
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
	url := meta.URL

	workDir, err := ioutil.TempDir("", "che-plugin-broker")
	if err != nil {
		return err
	}

	archivePath := filepath.Join(workDir, "testArchive.tar.gz")
	pluginPath := filepath.Join(workDir, "testArchive")

	// Download an archive
	log.Printf("Downloading archive '%s' to '%s'", url, archivePath)
	err = download(url, archivePath)
	if err != nil {
		return err
	}

	// Untar it
	log.Printf("Untarring '%s' to '%s'", archivePath, pluginPath)
	err = untar(archivePath, pluginPath)
	if err != nil {
		return err
	}

	log.Println("Resolving Che plugins")
	err = resolveToolingConfig(pluginPath)
	if err != nil {
		return err
	}

	log.Println("Copying dependencies")
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
			log.Printf("Copying file '%s' to '%s'", fileSrc, fileDest)
			if err = copyFile(fileSrc, fileDest); err != nil {
				return err
			}
		case dep.URL != "":
			fileDest := resolveDestPathFromURL(dep.URL, "/plugins")
			log.Printf("Downloading file '%s' to '%s'", dep.URL, fileDest)
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
