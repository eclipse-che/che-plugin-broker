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
	"log"
	"os"
	"path/filepath"

	"github.com/eclipse/che-plugin-broker/cfg"
	"github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/model"
)

func main() {
	log.SetOutput(os.Stdout)

	cfg.Parse()
	cfg.Print()

	broker := common.NewBroker()
	defer broker.CloseConsumers()

	if !cfg.DisablePushingToEndpoint {
		statusTun := common.ConnectOrFail(cfg.PushStatusesEndpoint, cfg.Token)
		broker.PushEvents(statusTun, model.BrokerLogEventType)
	}

	var filesToRemove []string
	broker.PrintInfo("Starting Init Plugin Broker")
	// Clear any existing plugins from /plugins/
	broker.PrintInfo("Cleaning /plugins dir")
	files, err := filepath.Glob(filepath.Join("/plugins", "*"))
	if err != nil {
		// Send log about clearing failure but proceed.
		// We might want to change this behavior later
		broker.PrintInfo("WARN: failed to clear /plugins directory. Error: %s", err)
		return
	}
	filesToRemove = append(filesToRemove, files...)
	broker.PrintInfo("Cleaning /sidecar-plugins dir")
	files, err = filepath.Glob(filepath.Join("/sidecar-plugins", "*"))
	if err != nil {
		// Send log about clearing failure but proceed.
		// We might want to change this behavior later
		broker.PrintInfo("WARN: failed to clear /sidecar-plugins directory. Error: %s", err)
		return
	}
	filesToRemove = append(filesToRemove, files...)

	for _, file := range filesToRemove {
		err = os.RemoveAll(file)
		if err != nil {
			broker.PrintInfo("WARN: failed to remove '%s'. Error: %s", file, err)
		}
	}
}
