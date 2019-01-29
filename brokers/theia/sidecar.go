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
	"github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/model"
	"regexp"
	"strconv"
)

func GenerateSidecarTooling(image string, pj model.PackageJSON, rand common.Random) *model.ToolingConf {
	tooling := &model.ToolingConf{
		Containers: []model.Container{*containerConfig(image, rand)},
	}
	addPortToTooling(tooling, pj, rand)

	return tooling
}

func containerConfig(image string, rand common.Random) *model.Container {
	c := model.Container{
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
	return &c
}

// addPortToTooling adds to tooling everything needed to start Theia remote plugin:
// - Random port to the container (one and only)
// - Endpoint matching the port
// - Environment variable THEIA_PLUGIN_ENDPOINT_PORT to the container with the port as value
// - Environment variable that start from THEIA_PLUGIN_REMOTE_ENDPOINT_ and ends with
// plugin publisher and plugin name taken from packageJson and replacing all
// chars matching [^a-z_0-9]+ with a dash character
func addPortToTooling(toolingConf *model.ToolingConf, pj model.PackageJSON, rand common.Random) {
	port := rand.IntFromRange(4000, 10000)
	sPort := strconv.Itoa(port)
	endpointName := rand.String(10)
	var re = regexp.MustCompile(`[^a-zA-Z_0-9]+`)
	prettyID := re.ReplaceAllString(pj.Publisher+"_"+pj.Name, `_`)
	theiaEnvVar1 := "THEIA_PLUGIN_REMOTE_ENDPOINT_" + prettyID
	theiaEnvVarValue := "ws://" + endpointName + ":" + sPort

	toolingConf.Containers[0].Ports = append(toolingConf.Containers[0].Ports, model.ExposedPort{ExposedPort: port})
	toolingConf.Endpoints = append(toolingConf.Endpoints, model.Endpoint{
		Name:       endpointName,
		Public:     false,
		TargetPort: port,
	})
	toolingConf.Containers[0].Env = append(toolingConf.Containers[0].Env, model.EnvVar{Name: "THEIA_PLUGIN_ENDPOINT_PORT", Value: sPort})
	toolingConf.WorkspaceEnv = append(toolingConf.WorkspaceEnv, model.EnvVar{Name: theiaEnvVar1, Value: theiaEnvVarValue})
}
