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

	statusTun := common.ConnectOrFail(cfg.PushStatusesEndpoint, cfg.Token)
	broker := common.NewBroker()
	defer broker.CloseConsumers()
	broker.PushEvents(statusTun, model.BrokerLogEventType)

	// Clear any existing plugins from /plugins/
	log.Println("Cleaning /plugins dir")
	files, err := filepath.Glob(filepath.Join("/plugins", "*"))
	if err != nil {
		// Send log about clearing failure but proceed.
		// We might want to change this behavior later
		broker.PrintInfo("WARN: failed to clear /plugins directory. Error: %s", err)
		return
	}

	for _, file := range files {
		err = os.RemoveAll(file)
		if err != nil {
			broker.PrintInfo("WARN: failed to remove '%s'. Error: %s", file, err)
		}
	}
}
