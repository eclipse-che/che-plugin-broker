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

	"github.com/eclipse/che-plugin-broker/broker"
	"github.com/eclipse/che-plugin-broker/cfg"
)

func main() {
	log.SetOutput(os.Stdout)

	cfg.Parse()
	cfg.Print()

	statusTun := broker.ConnectOrFail(cfg.PushStatusesEndpoint, cfg.Token)
	broker.PushEvents(statusTun)

	metas := cfg.ReadConfig()
	broker.Start(metas)
}
