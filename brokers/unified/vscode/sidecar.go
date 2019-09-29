//
// Copyright (c) 2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package vscode

import (
	"strconv"

	"github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/model"
)

// AddPluginRunnerRequirements adds to ChePlugin configuration needed to run remote Theia plugins in the provided ChePlugin.
// Method adds needed ports, endpoints, volumes, environment variables.
// ChePlugin with one container is supported only.
func AddPluginRunnerRequirements(plugin model.ChePlugin, rand common.Random, useLocalhost bool) model.ChePlugin {
	// TODO limitation is one and only sidecar
	container := &plugin.Containers[0]
	container.Volumes = append(container.Volumes, model.Volume{
		Name:      "plugins",
		MountPath: "/plugins",
	})

	container.MountSources = true
	if !useLocalhost {
		endpoint := generateTheiaSidecarEndpoint(rand)
		port := endpoint.TargetPort
		container.Ports = append(container.Ports, model.ExposedPort{ExposedPort: port})
		// TODO validate that there is no endpoints yet
		plugin.Endpoints = append(plugin.Endpoints, endpoint)
		container.Env = append(container.Env, model.EnvVar{
			Name:  "THEIA_PLUGIN_ENDPOINT_PORT",
			Value: strconv.Itoa(port),
		})
	}
	container.Env = append(container.Env, model.EnvVar{
		Name:  "THEIA_PLUGINS",
		Value: "local-dir:///plugins/sidecars/" + getPluginUniqueName(plugin),
	})

	return plugin
}

// AddExtension adds to ChePlugin an environment variable needed for extension to be consumed by Theia.
// Environment variable uses plugin name and publisher and version.
// Extension publisher and plugin name taken by retrieving info from package.json and replacing all
// chars matching [^a-z_0-9]+ with an underscore `_` character
// ChePlugin with a single endpoint is supported only.
func AddExtension(plugin model.ChePlugin) model.ChePlugin {
	// TODO limitation to have just one endpoint
	sidecarEndpoint := plugin.Endpoints[0]
	prettyID := getPluginUniqueName(plugin)
	sidecarTheiaEnvVarName := "THEIA_PLUGIN_REMOTE_ENDPOINT_" + prettyID
	sidecarTheiaEnvVarValue := "ws://" + sidecarEndpoint.Name + ":" + strconv.Itoa(sidecarEndpoint.TargetPort)
	plugin.WorkspaceEnv = append(plugin.WorkspaceEnv, model.EnvVar{Name: sidecarTheiaEnvVarName, Value: sidecarTheiaEnvVarValue})

	return plugin
}

// Generates random non-publicly exposed endpoint for sidecar to allow Theia connecting to it
func generateTheiaSidecarEndpoint(rand common.Random) model.Endpoint {
	endpointName := rand.String(10)
	port := rand.IntFromRange(4000, 10000)
	return model.Endpoint{
		Name:       endpointName,
		Public:     false,
		TargetPort: port,
	}
}
