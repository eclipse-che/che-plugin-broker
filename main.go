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
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/eclipse/che-plugin-broker/api"
	"github.com/eclipse/che/agents/go-agents/core/rest"
)

var (
	config = &brokerConfig{}
)

func init() {
	config.init()
}

// TODO: process logs, send them to master to show on WS start
func main() {
	flag.Parse()

	log.SetOutput(os.Stdout)

	config.printAll()

	appHTTPRoutes := []rest.RoutesGroup{
		api.HTTPRoutes,
	}

	// register routes and http handlers
	r := rest.NewDefaultRouter("", appHTTPRoutes)
	rest.PrintRoutes(appHTTPRoutes)

	server := &http.Server{
		Handler:      r,
		Addr:         config.serverAddress,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}

type brokerConfig struct {
	serverAddress string
}

func (cfg *brokerConfig) init() {
	// server configuration
	flag.StringVar(
		&cfg.serverAddress,
		"addr",
		":9000",
		"IP:PORT or :PORT the address to start the server on",
	)
}

func (cfg *brokerConfig) printAll() {
	log.Println("Plugin broker configuration")
	log.Println("  Server")
	log.Printf("    - Address: %s\n", cfg.serverAddress)
	log.Println()
}
