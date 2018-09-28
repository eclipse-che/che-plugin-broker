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

package cfg

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/eclipse/che-plugin-broker/model"
)

var (
	// FilePath path to config file.
	FilePath string

	// PushStatusesEndpoint where to push statuses.
	PushStatusesEndpoint string

	// AuthEnabled whether authentication is needed
	AuthEnabled bool

	// Token to access wsmaster API
	Token string

	// RuntimeID the id of workspace runtime this machine belongs to.
	RuntimeID    model.RuntimeID
	runtimeIDRaw string
)

func init() {
	curDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	flag.StringVar(
		&FilePath,
		"metas",
		curDir+string(os.PathSeparator)+"config.json",
		"Path to configuration file on filesystem",
	)
	flag.StringVar(
		&PushStatusesEndpoint,
		"push-endpoint",
		"",
		"WebSocket endpoint where to push statuses",
	)
	// auth configuration
	defaultAuthEnabled := false
	authEnabledEnv := os.Getenv("CHE_AUTH_ENABLED")
	b, e := strconv.ParseBool(authEnabledEnv)
	if e == nil {
		defaultAuthEnabled = b
	}
	flag.BoolVar(
		&AuthEnabled,
		"enable-auth",
		defaultAuthEnabled,
		"whether authenticate requests on workspace master before allowing them to proceed."+
			"By default the value from 'CHE_AUTH_ENABLED' environment variable is used or `false` if it is missing",
	)
	flag.StringVar(
		&runtimeIDRaw,
		"runtime-id",
		"",
		"The identifier of the runtime in format 'workspace:environment:ownerId'",
	)
}

// Parse parses configuration.
func Parse() {
	flag.Parse()

	// push-endpoint
	if len(PushStatusesEndpoint) == 0 {
		log.Fatal("Push endpoint required(set it with -push-endpoint argument)")
	}
	if !strings.HasPrefix(PushStatusesEndpoint, "ws") {
		log.Fatal("Push endpoint protocol must be either ws or wss")
	}

	// auth-enabled - fetch CHE_MACHINE_TOKEN
	if AuthEnabled {
		Token = os.Getenv("CHE_MACHINE_TOKEN")
	}

	// runtime-id
	if len(runtimeIDRaw) == 0 {
		log.Fatal("Runtime ID required(set it with -runtime-id argument)")
	}
	parts := strings.Split(runtimeIDRaw, ":")
	if len(parts) != 3 {
		log.Fatalf("Expected runtime id to be in format 'workspace:env:ownerId'")
	}
	RuntimeID = model.RuntimeID{Workspace: parts[0], Environment: parts[1], OwnerId: parts[2]}
}

// Print prints configuration.
func Print() {
	log.Print("Broker configuration")
	log.Printf("  Push endpoint: %s", PushStatusesEndpoint)
	log.Printf("  Auth enabled: %t", AuthEnabled)
	log.Print("  Runtime ID:")
	log.Printf("    Workspace: %s", RuntimeID.Workspace)
	log.Printf("    Environment: %s", RuntimeID.Environment)
	log.Printf("    OwnerId: %s", RuntimeID.OwnerId)

}

// ReadConfig reads content of file by path cfg.FilePath,
// parses its content as array of Che plugin meta objects and returns it.
// If any error occurs during read, log.Fatal is called.
func ReadConfig() []model.PluginMeta {
	f, err := os.Open(FilePath)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("Can't close Che plugins metas source, cause: %s", err)
		}
	}()

	raw, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}

	metas := make([]model.PluginMeta, 0)
	if err := json.Unmarshal(raw, &metas); err != nil {
		log.Fatal(err)
	}
	return metas
}
