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
	"github.com/eclipse/che-plugin-broker/common"
	cmock "github.com/eclipse/che-plugin-broker/common/mocks"
	"strconv"
	"testing"

	"github.com/eclipse/che-plugin-broker/model"
	"github.com/stretchr/testify/assert"
)

const (
	defaultEndpointName = "TestEndpointName"
	defaultPort         = 8889
)

func TestAddPluginRunnerRequirements(t *testing.T) {
	type args struct {
		plugin model.ChePlugin
		rand   common.Random
	}
	tests := []struct {
		name string
		args args
		want model.ChePlugin
	}{
		{
			name: "Check that all configuration needed for plugin runner is added",
			args: args{
				plugin: generateDefaultTestChePlugin(),
				rand:   generateMockOfRandom("newTestEndpoint", 4040),
			},
			want: model.ChePlugin{
				Containers: []model.Container{
					{
						Name:  "pluginsidecar",
						Image: "test/test:latest",
						Volumes: []model.Volume{
							{
								Name:      "plugins",
								MountPath: "/plugins",
							},
						},
						MountSources: true,
						Ports: []model.ExposedPort{
							{
								ExposedPort: 4040,
							},
						},
						Env: []model.EnvVar{
							{
								Name:  "THEIA_PLUGIN_ENDPOINT_PORT",
								Value: strconv.Itoa(4040),
							},
						},
					},
				},
				Endpoints: []model.Endpoint{
					{
						Name:       "newTestEndpoint",
						Public:     false,
						TargetPort: 4040,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := AddPluginRunnerRequirements(tt.args.plugin, tt.args.rand)
			assert.Equal(t, tt.want, actual)
		})
	}
}

func TestAddExtension(t *testing.T) {
	type args struct {
		plugin model.ChePlugin
		pj     PackageJSON
	}
	tests := []struct {
		name string
		args args
		want model.ChePlugin
	}{
		{
			name: "Test adding extension with package.json data with a-zA-Z0-9_ symbols",
			args: args{
				plugin: generateDefaultTestChePluginWithPluginRunnerConfig(),
				pj:     generatePackageJSON("pluginName8", "publisherName1_0_"),
			},
			want: generateDefaultTestChePluginWithPluginRunnerConfigWithExtension("pluginName8", "publisherName1_0_"),
		},
		{
			name: "Test adding extension with package.json data with # symbol",
			args: args{
				plugin: generateDefaultTestChePluginWithPluginRunnerConfig(),
				pj:     generatePackageJSON("plugin#Name8", "publisherName1_0_"),
			},
			want: generateDefaultTestChePluginWithPluginRunnerConfigWithExtension("plugin_Name8", "publisherName1_0_"),
		},
		{
			name: "Test adding extension with package.json data with @ symbol",
			args: args{
				plugin: generateDefaultTestChePluginWithPluginRunnerConfig(),
				pj:     generatePackageJSON("plu@ginName8", "publisherName1_0_"),
			},
			want: generateDefaultTestChePluginWithPluginRunnerConfigWithExtension("plu_ginName8", "publisherName1_0_"),
		},
		{
			name: "Test adding extension with package.json data with : symbol",
			args: args{
				plugin: generateDefaultTestChePluginWithPluginRunnerConfig(),
				pj:     generatePackageJSON("pluginName8", "publisherName:1_0_"),
			},
			want: generateDefaultTestChePluginWithPluginRunnerConfigWithExtension("pluginName8", "publisherName_1_0_"),
		},
		{
			name: "Test adding extension with package.json data with ? symbol",
			args: args{
				plugin: generateDefaultTestChePluginWithPluginRunnerConfig(),
				pj:     generatePackageJSON("pluginName8", "publisherName?1_0_"),
			},
			want: generateDefaultTestChePluginWithPluginRunnerConfigWithExtension("pluginName8", "publisherName_1_0_"),
		},
		{
			name: "Test adding extension with package.json data with - symbol",
			args: args{
				plugin: generateDefaultTestChePluginWithPluginRunnerConfig(),
				pj:     generatePackageJSON("plugin-Name-8", "publisherName1_0_"),
			},
			want: generateDefaultTestChePluginWithPluginRunnerConfigWithExtension("plugin_Name_8", "publisherName1_0_"),
		},
		{
			name: "Test adding extension with package.json data with ! symbol",
			args: args{
				plugin: generateDefaultTestChePluginWithPluginRunnerConfig(),
				pj:     generatePackageJSON("plugin!Name8", "publisherName1_0_!"),
			},
			want: generateDefaultTestChePluginWithPluginRunnerConfigWithExtension("plugin_Name8", "publisherName1_0__"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := AddExtension(tt.args.plugin, tt.args.pj)
			assert.Equal(t, actual, tt.want)
		})
	}
}

func TestAddSeveralExtensions(t *testing.T) {
	plugin := generateDefaultTestChePluginWithPluginRunnerConfig()
	pj := generatePackageJSON("pluginName1", "publisherName1_0_")
	expected := generateDefaultTestChePluginWithPluginRunnerConfigWithExtension("pluginName1", "publisherName1_0_")

	actual := AddExtension(plugin, pj)
	assert.Equal(t, actual, expected)

	pj2 := generatePackageJSON("pluginName2", "publisherName2_0_")
	expected2 := model.ChePlugin(expected)
	expected2.WorkspaceEnv = append(expected2.WorkspaceEnv, model.EnvVar{
		Name:  "THEIA_PLUGIN_REMOTE_ENDPOINT_" + "publisherName2_0_" + "_" + "pluginName2",
		Value: "ws://" + defaultEndpointName + ":" + strconv.Itoa(defaultPort),
	})
	actual2 := AddExtension(actual, pj2)
	assert.Equal(t, actual2, expected2)
}

func generatePackageJSON(name string, publisher string) PackageJSON {
	return PackageJSON{
		Name:      name,
		Publisher: publisher,
	}
}

func generateDefaultTestChePluginWithPluginRunnerConfigWithExtension(extName string, extPublisher string) model.ChePlugin {
	plugin := generateTestChePluginWithPluginRunnerConfig(defaultPort, defaultEndpointName)
	plugin.WorkspaceEnv = append(plugin.WorkspaceEnv, model.EnvVar{
		Name:  "THEIA_PLUGIN_REMOTE_ENDPOINT_" + extPublisher + "_" + extName,
		Value: "ws://" + defaultEndpointName + ":" + strconv.Itoa(defaultPort),
	})
	return plugin
}

func generateDefaultTestChePluginWithPluginRunnerConfig() model.ChePlugin {
	return generateTestChePluginWithPluginRunnerConfig(defaultPort, defaultEndpointName)
}

func generateTestChePluginWithPluginRunnerConfig(port int, endpointName string) model.ChePlugin {
	return model.ChePlugin{
		Containers: []model.Container{
			{
				Name:  "pluginsidecar",
				Image: "test/test:latest",
				Volumes: []model.Volume{
					{
						Name:      "plugins",
						MountPath: "/plugins",
					},
				},
				MountSources: true,
				Ports: []model.ExposedPort{
					{
						ExposedPort: port,
					},
				},
				Env: []model.EnvVar{
					{
						Name:  "THEIA_PLUGIN_ENDPOINT_PORT",
						Value: strconv.Itoa(port),
					},
				},
			},
		},
		Endpoints: []model.Endpoint{
			{
				Name:       endpointName,
				Public:     false,
				TargetPort: port,
			},
		},
	}
}

func generateDefaultTestChePlugin() model.ChePlugin {
	return model.ChePlugin{
		Containers: []model.Container{
			{
				Name:         "pluginsidecar",
				Image:        "test/test:latest",
				MountSources: false,
			},
		},
	}
}

func generateMockOfRandom(expectedString string, expectedInt int) common.Random {
	rand := &cmock.Random{}
	rand.On("String", 10).Return(expectedString)
	rand.On("IntFromRange", 4000, 10000).Return(expectedInt)
	return rand
}
