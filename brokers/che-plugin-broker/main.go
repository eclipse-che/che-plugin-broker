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

	"github.com/eclipse/che-plugin-broker/brokers/che-plugin-broker/broker"
	"github.com/eclipse/che-plugin-broker/cfg"
	"github.com/eclipse/che-plugin-broker/common"
)

func main() {
	log.SetOutput(os.Stdout)

	cfg.Parse()
	cfg.Print()

	statusTun := common.ConnectOrFail(cfg.PushStatusesEndpoint, cfg.Token)
	cheBroker := broker.NewBroker()
	cheBroker.PushEvents(statusTun)

	metas := cfg.ReadConfig()
	cheBroker.Start(metas)
}
