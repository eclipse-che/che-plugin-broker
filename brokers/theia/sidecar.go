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

package theia

import (
	"regexp"
	"strconv"

	"github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/model"
)

var re = regexp.MustCompile(`[^a-zA-Z_0-9]+`)

// GenerateSidecarTooling generates Theia plugin runner sidecar and adds a single plugin to it
// Deprecated: use GenerateSidecar + AddExtension instead
func GenerateSidecarTooling(image string, pj model.PackageJSON, rand common.Random) *model.ToolingConf {
	tooling := &model.ToolingConf{}
	tooling.Containers = append(tooling.Containers, containerConfig(image, rand))
	endpoint := generateTheiaSidecarEndpoint(rand)
	setEndpoint(tooling, endpoint)
	AddExtension(tooling, pj)

	return tooling
}

// GenerateSidecar generates sidecar tooling configuration.
// Plugins can be added to the configuration using function AddExtension
func GenerateSidecar(image string, rand common.Random) *model.ToolingConf {
	tooling := &model.ToolingConf{}
	tooling.Containers = append(tooling.Containers, containerConfig(image, rand))
	endpoint := generateTheiaSidecarEndpoint(rand)
	setEndpoint(tooling, endpoint)

	return tooling
}

// AddExtension adds to tooling an environment variable needed for extension to be consumed by Theia.
// Environment variable uses extension name and publisher specified in PackageJSON.
// Extension publisher and plugin name taken by retrieving info from package.json and replacing all
// chars matching [^a-z_0-9]+ with a dash character
func AddExtension(toolingConf *model.ToolingConf, pj model.PackageJSON) {
	sidecarEndpoint := toolingConf.Endpoints[0]
	prettyID := re.ReplaceAllString(pj.Publisher+"_"+pj.Name, `_`)
	sidecarTheiaEnvVarName := "THEIA_PLUGIN_REMOTE_ENDPOINT_" + prettyID
	sidecarTheiaEnvVarValue := "ws://" + sidecarEndpoint.Name + ":" + strconv.Itoa(sidecarEndpoint.TargetPort)

	toolingConf.WorkspaceEnv = append(toolingConf.WorkspaceEnv, model.EnvVar{Name: sidecarTheiaEnvVarName, Value: sidecarTheiaEnvVarValue})
}

// Generates sidecar container config with needed image and volumes
func containerConfig(image string, rand common.Random) model.Container {
	return model.Container{
		Name:  "pluginsidecar" + rand.String(6),
		Image: image,
		Volumes: []model.Volume{
			{
				Name:      "projects",
				MountPath: "/projects",
			},
			{
				Name:      "plugins",
				MountPath: "/plugins",
			},
		},
	}
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

// Sets sidecar endpoint into tooling and adds needed port exposure and environment variable to the sidecar container
// to run plugin runner Theia slave on a port specified in provided endpoint
func setEndpoint(toolingConf *model.ToolingConf, endpoint model.Endpoint) {
	port := endpoint.TargetPort
	toolingConf.Containers[0].Ports = append(toolingConf.Containers[0].Ports, model.ExposedPort{ExposedPort: port})
	toolingConf.Endpoints = append(toolingConf.Endpoints, endpoint)
	toolingConf.Containers[0].Env = append(toolingConf.Containers[0].Env, model.EnvVar{Name: "THEIA_PLUGIN_ENDPOINT_PORT", Value: strconv.Itoa(port)})
}
