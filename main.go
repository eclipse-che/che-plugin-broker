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

	"github.com/gin-gonic/gin"
	"os/signal"
	"context"
	"github.com/eclipse/che-plugin-broker/api"
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

	router := gin.Default()


	api.SetUpRouter(router)


	srv := &http.Server{
		Addr:    config.serverAddress,
		Handler: router,
	}

	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
	log.Println("Server exiting")

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
