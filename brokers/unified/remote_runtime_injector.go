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
	CheTheiaEditorName = "che-theia"

	InjectorContainerName = "remote-runtime-injector"

	RemoteEndpointExecutableEnvVar = "PLUGIN_REMOTE_ENDPOINT_EXECUTABLE"
	VolumeNameEnvVar               = "REMOTE_ENDPOINT_VOLUME_NAME"
)

type RemotePluginInjection struct {
	Volume model.Volume
	Env    model.EnvVar
}

func InjectRemoteRuntime(plugins []model.ChePlugin) error {
	editorPlugin := findCheTheiaEditor(plugins)
	if editorPlugin == nil {
		return nil
	}

	injection, err := getRuntimeInjection(editorPlugin)
	if err != nil {
		return err
	}

	for _, plugin := range plugins {
		inject(&plugin, injection)
	}

	return nil
}

func findCheTheiaEditor(plugins []model.ChePlugin) *model.ChePlugin {
	for _, plugin := range plugins {
		if strings.ToLower(plugin.Type) == model.EditorPluginType &&
			strings.ToLower(plugin.Name) == CheTheiaEditorName &&
			len(plugin.InitContainers) > 0 {
			return &plugin
		}
	}
	// it's ok, maybe used some another editor instead of che-theia
	return nil
}

func getRuntimeInjection(editorPlugin *model.ChePlugin) (*RemotePluginInjection, error) {
	containerInjector, err := findContainerInjector(editorPlugin.InitContainers)
	if err != nil {
		// it's ok, older che-theia could be without runtime injection
		return nil, nil
	}

	runtimeBinaryPathEnv, err := findEnv(RemoteEndpointExecutableEnvVar, containerInjector.Env)
	if err != nil {
		return nil, err
	}

	volumeName, err := findEnv(VolumeNameEnvVar, containerInjector.Env)
	if err != nil {
		return nil, err
	}

	volume, err := findVolume(volumeName.Value, containerInjector.Volumes)
	if err != nil {
		return nil, err
	}

	return &RemotePluginInjection{
		Volume: *volume,
		Env:    *runtimeBinaryPathEnv,
	}, nil
}

func findContainerInjector(containers []model.Container) (*model.Container, error) {
	for _, container := range containers {
		if container.Name == InjectorContainerName {
			return &container, nil
		}
	}
	return nil, errors.New("Unable to find injector container")
}

func findEnv(envName string, envVars []model.EnvVar) (*model.EnvVar, error) {
	var result *model.EnvVar
	for _, envVar := range envVars {
		if envVar.Name == envName {
			result = &envVar
			break
		}
	}
	if result == nil {
		return nil, errors.New("Unable to find required env with name " + envName)
	}
	if result.Value == "" {
		return nil, errors.New("Required env with name " + envName + " was found, but value is empty")
	}

	return result, nil
}

func findVolume(volumeName string, volumes []model.Volume) (*model.Volume, error) {
	for _, volume := range volumes {
		if volume.Name == volumeName {
			return &volume, nil
		}
	}
	return nil, errors.New("Unable to find volume by name " + volumeName)
}

func inject(plugin *model.ChePlugin, injection *RemotePluginInjection) {
	pluginType := strings.ToLower(plugin.Type)

	if pluginType != model.TheiaPluginType && pluginType != model.VscodePluginType {
		return
	}
	// sidecar container has one and only one container.
	container := &plugin.Containers[0]

	container.Env = append(container.Env, injection.Env)
	container.Volumes = append(container.Volumes, injection.Volume)
}
