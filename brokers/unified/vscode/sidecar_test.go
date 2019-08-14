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
	"strings"
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
		plugin       model.ChePlugin
		rand         common.Random
		useLocalhost bool
	}
	tests := []struct {
		name string
		args args
		want model.ChePlugin
	}{
		{
			name: "Check that all configuration needed for plugin runner is added, when plugins are not on the Theia host",
			args: args{
				plugin: generateDefaultTestChePlugin(),
				rand:   generateMockOfRandom("newTestEndpoint", 4040),
			},
			want: model.ChePlugin{
				Publisher: pluginPublisher,
				Name:      pluginName,
				Version:   pluginVersion,
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
							{Name: "THEIA_PLUGINS",
								Value: "local-dir:///plugins/sidecars/test_publisher_test_name_tv",
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
		{
			name: "Check that all configuration needed for plugin runner is added, when plugins are on the Theia host",
			args: args{
				plugin:       generateDefaultTestChePlugin(),
				rand:         generateMockOfRandom("newTestEndpoint", 4040),
				useLocalhost: true,
			},
			want: model.ChePlugin{
				Publisher: pluginPublisher,
				Name:      pluginName,
				Version:   pluginVersion,
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
						Env: []model.EnvVar{
							{Name: "THEIA_PLUGINS",
								Value: "local-dir:///plugins/sidecars/test_publisher_test_name_tv",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := AddPluginRunnerRequirements(tt.args.plugin, tt.args.rand, tt.args.useLocalhost)
			assert.Equal(t, tt.want, actual)
		})
	}
}

func TestAddExtension(t *testing.T) {
	type args struct {
		plugin model.ChePlugin
	}
	tests := []struct {
		name string
		args args
		want model.ChePlugin
	}{
		{
			name: "Test adding extension",
			args: args{
				plugin: model.ChePlugin{
					Publisher: pluginPublisher,
					Name:      pluginName,
					Version:   pluginVersion,
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
									ExposedPort: defaultPort,
								},
							},
							Env: []model.EnvVar{
								{
									Name:  "THEIA_PLUGIN_ENDPOINT_PORT",
									Value: strconv.Itoa(defaultPort),
								},
							},
						},
					},
					Endpoints: []model.Endpoint{
						{
							Name:       defaultEndpointName,
							Public:     false,
							TargetPort: defaultPort,
						},
					},
				},
			},
			want: model.ChePlugin{
				Publisher: pluginPublisher,
				Name:      pluginName,
				Version:   pluginVersion,
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
								ExposedPort: defaultPort,
							},
						},
						Env: []model.EnvVar{
							{
								Name:  "THEIA_PLUGIN_ENDPOINT_PORT",
								Value: strconv.Itoa(defaultPort),
							},
						},
					},
				},
				Endpoints: []model.Endpoint{
					{
						Name:       defaultEndpointName,
						Public:     false,
						TargetPort: defaultPort,
					},
				},
				WorkspaceEnv: []model.EnvVar{
					model.EnvVar{
						Name:  "THEIA_PLUGIN_REMOTE_ENDPOINT_" + strings.ReplaceAll(pluginPublisher+"_"+pluginName+"_"+pluginVersion, " ", "_"),
						Value: "ws://" + defaultEndpointName + ":" + strconv.Itoa(defaultPort),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := AddExtension(tt.args.plugin)
			assert.Equal(t, actual, tt.want)
		})
	}
}

func generateDefaultTestChePlugin() model.ChePlugin {
	return model.ChePlugin{
		Publisher: pluginPublisher,
		Name:      pluginName,
		Version:   pluginVersion,
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
