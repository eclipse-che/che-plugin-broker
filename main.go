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
	"time"

	"github.com/eclipse/che-plugin-broker/broker"
	"github.com/eclipse/che-plugin-broker/cfg"
)

func main() {
	log.SetOutput(os.Stdout)

	cfg.Parse()
	cfg.Print()

	broker.EndpointReconnectPeriod = time.Second * time.Duration(cfg.EndpointReconnectPeriodSec)

	statusTun := broker.ConnectOrFail(cfg.PushStatusesEndpoint, cfg.Token)
	broker.PushStatuses(statusTun)

	// in case cfg.PushLogsEndpoint is not specified cfg.PushStatusesEndpoint is used instead
	if len(cfg.PushLogsEndpoint) != 0 {
		connector := &broker.WSDialConnector{
			Endpoint: cfg.PushLogsEndpoint,
			Token:    cfg.Token,
		}
		if cfg.PushLogsEndpoint == cfg.PushStatusesEndpoint {
			broker.PushLogs(statusTun, connector)
		} else {
			broker.PushLogs(broker.ConnectOrFail(cfg.PushLogsEndpoint, cfg.Token), connector)
		}
	}

	metas := cfg.ReadConfig()
	broker.Start(metas)
}
