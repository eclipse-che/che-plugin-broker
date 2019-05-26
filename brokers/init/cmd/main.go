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

package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/eclipse/che-plugin-broker/cfg"
	"github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/model"
)

func recursive(parentWg *sync.WaitGroup, broker common.Broker, dir string, depth int) {
	defer func() {
		if parentWg != nil {
			err := os.RemoveAll(dir)
			if err != nil {
				broker.PrintInfo("WARN: failed to remove '%s'. Error: %s", dir, err)
			}
			parentWg.Done()
		}
	}()

	depth--
	if depth < 0 {
		return
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		broker.PrintInfo("WARN: failed to read dir '%s'. Error: %s", dir, err)
		return
	}

	wg := sync.WaitGroup{}
	for _, file := range files {
		if file.IsDir() {
			path := filepath.Join(dir, file.Name())
			wg.Add(1)
			go recursive(&wg, broker, path, depth)
		}
	}
	wg.Wait()
}

func main() {
        runtime.GOMAXPROCS(runtime.NumCPU())
	log.SetOutput(os.Stdout)

	cfg.Parse()
	cfg.Print()

	broker := common.NewBroker()
	defer broker.CloseConsumers()

	if !cfg.DisablePushingToEndpoint {
		statusTun := common.ConnectOrFail(cfg.PushStatusesEndpoint, cfg.Token)
		broker.PushEvents(statusTun, model.BrokerLogEventType)
	}

	broker.PrintInfo("Starting Init Plugin Broker")
	// Clear any existing plugins from /plugins/
	broker.PrintInfo("Cleaning /plugins dir")
	recursive(nil, broker, "/plugins", 1)
}
