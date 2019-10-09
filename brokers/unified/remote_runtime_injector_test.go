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
	"testing"

	"github.com/eclipse/che-plugin-broker/model"
	"github.com/stretchr/testify/assert"
)

const (
	ExecutablePathTest = "/some-path"
	VolumeNameTest     = "some-volume"
	VolumePathTest     = "/some-volume"
)

func TestShouldNotInjectRemotePluginRuntimeForChePluginType(t *testing.T) {
	editorPlugin := createEditorPluginWithRuntimeInjection()
	vscodePlugin := createPlugin(model.ChePluginType)

	plugins := []model.ChePlugin{*editorPlugin, *vscodePlugin}

	err := InjectRemoteRuntime(plugins)

	assert.Equal(t, plugins, []model.ChePlugin{*createEditorPluginWithRuntimeInjection(), *createPlugin(model.ChePluginType)})
	assert.Equal(t, err, nil)
}

func TestShouldNotInjectRemotePluginRuntimeWithEditorWithNoInitContainers(t *testing.T) {
	editorPlugin := createEditorPlugin()
	vscodePlugin := createPlugin(model.VscodePluginType)

	plugins := []model.ChePlugin{*editorPlugin, *vscodePlugin}

	err := InjectRemoteRuntime(plugins)

	assert.Equal(t, plugins, []model.ChePlugin{*createEditorPlugin(), *createPlugin(model.VscodePluginType)})
	assert.Equal(t, err, nil)
}

func TestShouldNotInjectRemotePluginRuntimeWhenNoEditor(t *testing.T) {
	vscodePlugin1 := createPlugin(model.VscodePluginType)
	vscodePlugin2 := createPlugin(model.VscodePluginType)

	plugins := []model.ChePlugin{*vscodePlugin1, *vscodePlugin2}

	err := InjectRemoteRuntime(plugins)

	assert.Equal(t, plugins, []model.ChePlugin{*createPlugin(model.VscodePluginType), *createPlugin(model.VscodePluginType)})
	assert.Equal(t, err, nil)
}

func TestShouldInjectRemotePluginRuntime(t *testing.T) {
	editorPlugin := createEditorPluginWithRuntimeInjection()
	vscodePlugin := createPlugin(model.VscodePluginType)

	plugins := []model.ChePlugin{*editorPlugin, *vscodePlugin}

	err := InjectRemoteRuntime(plugins)

	assert.Equal(t, plugins, []model.ChePlugin{*createEditorPluginWithRuntimeInjection(), *exectedVsCodePluginWithRuntimeInjection()})
	assert.Equal(t, err, nil)
}

func createEditorPluginWithRuntimeInjection() *model.ChePlugin {
	editorPlugin := createEditorPlugin()
	editorPlugin.InitContainers = []model.Container{
		{
			Image: "eclipse/che-theia-runtime-binary",
			Name:  InjectorContainerName,
			Env: []model.EnvVar{
				{
					Name:  RemoteEndpointExecutableEnvVar,
					Value: ExecutablePathTest,
				},
				{
					Name:  VolumeNameEnvVar,
					Value: VolumeNameTest,
				},
			},
			Volumes: []model.Volume{
				{
					Name:      VolumeNameTest,
					MountPath: VolumePathTest,
					Ephemeral: true,
				},
			},
		},
	}
	return editorPlugin
}

func createEditorPlugin() *model.ChePlugin {
	return &model.ChePlugin{
		ID:        "some-id-1",
		Version:   "latest",
		Name:      CheTheiaEditorName,
		Type:      model.EditorPluginType,
		Publisher: "eclipse",
		Containers: []model.Container{
			{
				Image: "eclipse/che-theia",
				Name:  "che-theia",
				Env:   []model.EnvVar{},
			},
		},
	}
}

func createPlugin(pluginType string) *model.ChePlugin {
	return &model.ChePlugin{
		ID:        "some-id-2",
		Version:   "latest",
		Name:      "vscode-xml",
		Type:      pluginType,
		Publisher: "eclipse",
		Containers: []model.Container{
			{
				Image: "eclipse/xml-lsp",
				Name:  "xml-lsp",
				Env: []model.EnvVar{
					{
						Name:  "Test",
						Value: "some-value",
					},
				},
				Volumes: []model.Volume{
					{
						Name:      "projects",
						MountPath: "/projects",
					},
				},
			},
		},
	}
}

func exectedVsCodePluginWithRuntimeInjection() *model.ChePlugin {
	return &model.ChePlugin{
		ID:        "some-id-2",
		Version:   "latest",
		Name:      "vscode-xml",
		Type:      model.VscodePluginType,
		Publisher: "eclipse",
		Containers: []model.Container{
			{
				Image: "eclipse/xml-lsp",
				Name:  "xml-lsp",
				Env: []model.EnvVar{
					{
						Name:  "Test",
						Value: "some-value",
					},
					{
						Name:  RemoteEndpointExecutableEnvVar,
						Value: ExecutablePathTest,
					},
				},
				Volumes: []model.Volume{
					{
						Name:      "projects",
						MountPath: "/projects",
					},
					{
						Name:      VolumeNameTest,
						MountPath: VolumePathTest,
						Ephemeral: true,
					},
				},
				Command: []string{ExecutablePathTest},
			},
		},
	}
}
