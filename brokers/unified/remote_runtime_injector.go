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

package unified

import (
	"errors"
	"strings"

	"github.com/eclipse/che-plugin-broker/model"
)

const (
	DefaultEditorName = "che-theia"

	InjectorContainerName = "remote-runtime-injector"

	RemoteEndPointVolume     = "remote-endpoint"
	RemoteEndPointVolumePath = "/remote-endpoint"

	RemoteEndPontExecutableEnvVar = "PLUGIN_REMOTE_ENDPOINT"
	RemoteEndPointExecutable      = "plugin-remote-endpoint"
	RemoteEndPointExecPath        = RemoteEndPointVolumePath + "/" + RemoteEndPointExecutable
)

type RemotePluginInjection struct {
	Volumes model.Volume
	Env     model.EnvVar
	Command []string
	Args    []string
}

func InjectRemoteRuntime(plugins []model.ChePlugin) {
	editorPlugin, err := findEditorPlugin(plugins)
	if err != nil {
		return
	}

	if !hasRuntimeContainerWithInjection(*editorPlugin) {
		return
	}

	injection := &RemotePluginInjection{
		Volumes: model.Volume{
			Name:      RemoteEndPointVolume,
			MountPath: RemoteEndPointVolumePath,
			Ephemeral: true,
		},
		Env: model.EnvVar{
			Name:  RemoteEndPontExecutableEnvVar,
			Value: RemoteEndPointExecPath,
		},
		Command: []string{RemoteEndPointExecPath},
	}
	for _, plugin := range plugins {
		inject(&plugin, injection)
	}
}

func findEditorPlugin(plugins []model.ChePlugin) (*model.ChePlugin, error) {
	for _, plugin := range plugins {
		if strings.ToLower(plugin.Type) == model.EditorPluginType &&
			strings.ToLower(plugin.Name) == DefaultEditorName &&
			len(plugin.InitContainers) > 0 {
			return &plugin, nil
		}
	}
	return nil, errors.New("Unable to find editor plugin")
}

func hasRuntimeContainerWithInjection(editorPlugin model.ChePlugin) bool {
	for _, initContainer := range editorPlugin.InitContainers {
		if initContainer.Name == InjectorContainerName {
			return true
		}
	}
	return false
}

func inject(plugin *model.ChePlugin, injection *RemotePluginInjection) {
	pluginType := strings.ToLower(plugin.Type)

	if pluginType != model.TheiaPluginType && pluginType != model.VscodePluginType {
		return
	}
	// sidecar container has one and only one container.
	container := &plugin.Containers[0]

	container.Env = append(container.Env, injection.Env)
	container.Volumes = append(container.Volumes, injection.Volumes)
	if len(container.Command) == 0 && len(container.Args) == 0 {
		container.Command = injection.Command
	}
}
