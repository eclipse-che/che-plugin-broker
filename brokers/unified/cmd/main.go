//
// Copyright (c) 2018-2019 Red Hat, Inc.
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

	"github.com/eclipse/che-plugin-broker/brokers/unified"
	"github.com/eclipse/che-plugin-broker/cfg"
	"github.com/eclipse/che-plugin-broker/common"
)

func main() {
	log.SetOutput(os.Stdout)

	cfg.Parse()
	cfg.Print()

	broker := unified.NewBroker()

	if !cfg.DisablePushingToEndpoint {
		statusTun := common.ConnectOrFail(cfg.PushStatusesEndpoint, cfg.Token)
		broker.PushEvents(statusTun)
	}

	if cfg.DownloadMetas {
		pluginFQNs, err := cfg.ParsePluginFQNs()
		if err != nil {
			broker.PrintFatal("Failed to process plugin fully qualified names from config: %s", err)
		}
		broker.DownloadMetasAndStart(pluginFQNs, cfg.RegistryAddress)
	} else {
		pluginMetas, err := cfg.ReadConfig()
		if err != nil {
			broker.PrintFatal("Failed to process plugin fully qualified names from config: %s", err)
		}
		broker.Start(pluginMetas)
	}
}
