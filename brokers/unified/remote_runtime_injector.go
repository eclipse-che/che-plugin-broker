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

	RemoteEndPontExecutableEnvVar = "PLUGIN_REMOTE_ENDPOINT_EXECUTABLE"
	VolumeNameEnvVar              = "REMOTE_ENDPOINT_VOLUME_NAME"
)

type RemotePluginInjection struct {
	Volume  model.Volume
	Env     model.EnvVar
	Command []string
	Args    []string
}

func InjectRemoteRuntime(plugins []model.ChePlugin) {
	editorPlugin, err := findDefaultEditorPlugin(plugins)
	if err != nil {
		return
	}

	injection, err := getRuntimeInjection(editorPlugin)
	if err != nil {
		return
	}

	for _, plugin := range plugins {
		inject(&plugin, injection)
	}
}

func findDefaultEditorPlugin(plugins []model.ChePlugin) (*model.ChePlugin, error) {
	for _, plugin := range plugins {
		if strings.ToLower(plugin.Type) == model.EditorPluginType &&
			strings.ToLower(plugin.Name) == DefaultEditorName &&
			len(plugin.InitContainers) > 0 {
			return &plugin, nil
		}
	}
	return nil, errors.New("Unable to find default editor plugin")
}

func getRuntimeInjection(editorPlugin *model.ChePlugin) (*RemotePluginInjection, error) {
	containerInjector, err := findContainerInjector(editorPlugin.InitContainers)
	if err != nil {
		return nil, err
	}

	runtimeBinaryPathEnv, err := findEnv(RemoteEndPontExecutableEnvVar, containerInjector.Env)
	if err != nil || runtimeBinaryPathEnv.Value == "" {
		return nil, err
	}

	volumeName, err := findEnv(VolumeNameEnvVar, containerInjector.Env)
	if err != nil || runtimeBinaryPathEnv.Value == "" {
		return nil, err
	}

	volume, err := findVolume(volumeName.Value, containerInjector.Volumes)
	if err != nil {
		return nil, err
	}

	return &RemotePluginInjection{
		Volume:  *volume,
		Env:     *runtimeBinaryPathEnv,
		Command: []string{runtimeBinaryPathEnv.Value},
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
	for _, envVar := range envVars {
		if envVar.Name == envName {
			return &envVar, nil
		}
	}
	return nil, errors.New("Unable to find env by name")
}

func findVolume(volumeName string, volumes []model.Volume) (*model.Volume, error) {
	for _, volume := range volumes {
		if volume.Name == volumeName {
			return &volume, nil
		}
	}
	return nil, errors.New("Unable to find volume by name")
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
