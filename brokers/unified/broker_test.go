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
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	vscodeBrokerMocks "github.com/eclipse/che-plugin-broker/brokers/unified/vscode/mocks"
	cmock "github.com/eclipse/che-plugin-broker/common/mocks"
	"github.com/eclipse/che-plugin-broker/model"
	storageMocks "github.com/eclipse/che-plugin-broker/storage/mocks"
	fmock "github.com/eclipse/che-plugin-broker/utils/mocks"
	"github.com/stretchr/testify/mock"
)

const TestChePluginType = "Che Plugin"
const TestEditorPluginType = "Che Editor"
const TestTheiaPluginType = "Theia plugin"
const TestVscodePluginType = "VS Code extension"
const defaultImage = "test-image:latest"

type mocks struct {
	cb           *cmock.Broker
	u            *fmock.IoUtil
	randMock     *cmock.Random
	b            *Broker
	vscodeBroker *vscodeBrokerMocks.Broker
	storage      *storageMocks.Storage
}

func createMocks() *mocks {
	cb := &cmock.Broker{}
	u := &fmock.IoUtil{}
	randMock := &cmock.Random{}
	vscodeBroker := &vscodeBrokerMocks.Broker{}
	storageMock := &storageMocks.Storage{}

	cb.On("PrintInfo", mock.AnythingOfType("string"))

	return &mocks{
		cb:       cb,
		u:        u,
		randMock: randMock,
		b: &Broker{
			Broker:       cb,
			Storage:      storageMock,
			utils:        u,
			vscodeBroker: vscodeBroker,
		},
		vscodeBroker: vscodeBroker,
		storage:      storageMock,
	}
}

func TestBroker_processPlugins(t *testing.T) {
	type args struct {
		metas []model.PluginMeta
	}
	type want struct {
		err           string
		vscodeMetas   []model.PluginMeta
		commonPlugins []model.ChePlugin
	}
	type mocks struct {
		vsCodeError  error
		storageError error
	}
	tests := []struct {
		name  string
		args  args
		mocks mocks
		want  want
	}{
		{
			name: "Sends error on VS Code broker error",
			mocks: mocks{
				vsCodeError: errors.New("test vscode error"),
			},
			args: args{
				metas: []model.PluginMeta{createDefaultTheiaMeta(), createDefaultVSCodeMeta(), createDefaultChePluginMeta()},
			},
			want: want{
				err: "test vscode error",
			},
		},
		{
			name: "Returns error when apiVersion is not specified in meta.yaml",
			args: args{
				metas: []model.PluginMeta{createDefaultVSCodeMetaWithApiVersion("", "id1111")},
			},
			want: want{
				err: "Plugin 'id1111' is invalid. Field 'apiVersion' must be present",
			},
		},
		{
			name: "Returns error when apiVersion is not specified in meta.yaml",
			args: args{
				metas: []model.PluginMeta{createDefaultVSCodeMetaWithApiVersion("v1", "id2111")},
			},
			want: want{
				err: "Plugin 'id2111' is invalid. Field 'apiVersion' contains invalid version 'v1'",
			},
		},
		{
			name: "Returns error when apiVersion is not specified in meta.yaml",
			args: args{
				metas: []model.PluginMeta{createDefaultVSCodeMetaWithApiVersion("v100", "id3111")},
			},
			want: want{
				err: "Plugin 'id3111' is invalid. Field 'apiVersion' contains invalid version 'v100'",
			},
		},
		{
			name: "Returns error when apiVersion is not specified in meta.yaml",
			args: args{
				metas: []model.PluginMeta{createDefaultTheiaMetaWithApiVersion("", "id111")},
			},
			want: want{
				err: "Plugin 'id111' is invalid. Field 'apiVersion' must be present",
			},
		},
		{
			name: "Returns error when apiVersion is not specified in meta.yaml",
			args: args{
				metas: []model.PluginMeta{createDefaultTheiaMetaWithApiVersion("v1", "id211")},
			},
			want: want{
				err: "Plugin 'id211' is invalid. Field 'apiVersion' contains invalid version 'v1'",
			},
		},
		{
			name: "Returns error when apiVersion is not specified in meta.yaml",
			args: args{
				metas: []model.PluginMeta{createDefaultTheiaMetaWithApiVersion("v100", "id311")},
			},
			want: want{
				err: "Plugin 'id311' is invalid. Field 'apiVersion' contains invalid version 'v100'",
			},
		},
		{
			name: "Returns error when apiVersion is not specified in meta.yaml",
			args: args{
				metas: []model.PluginMeta{createDefaultChePluginMetaWithApiVersion("", "id11")},
			},
			want: want{
				err: "Plugin 'id11' is invalid. Field 'apiVersion' must be present",
			},
		},
		{
			name: "Returns error when apiVersion is not specified in meta.yaml",
			args: args{
				metas: []model.PluginMeta{createDefaultChePluginMetaWithApiVersion("v1", "id21")},
			},
			want: want{
				err: "Plugin 'id21' is invalid. Field 'apiVersion' contains invalid version 'v1'",
			},
		},
		{
			name: "Returns error when apiVersion is not specified in meta.yaml",
			args: args{
				metas: []model.PluginMeta{createDefaultChePluginMetaWithApiVersion("v100", "id31")},
			},
			want: want{
				err: "Plugin 'id31' is invalid. Field 'apiVersion' contains invalid version 'v100'",
			},
		},
		{
			name: "Returns error when apiVersion is not specified in meta.yaml",
			args: args{
				metas: []model.PluginMeta{createDefaultCheEditorMetaWithApiVersion("", "id1")},
			},
			want: want{
				err: "Plugin 'id1' is invalid. Field 'apiVersion' must be present",
			},
		},
		{
			name: "Returns error when apiVersion is not specified in meta.yaml",
			args: args{
				metas: []model.PluginMeta{createDefaultCheEditorMetaWithApiVersion("v1", "id2")},
			},
			want: want{
				err: "Plugin 'id2' is invalid. Field 'apiVersion' contains invalid version 'v1'",
			},
		},
		{
			name: "Returns error when apiVersion in meta.yaml is unsupported",
			args: args{
				metas: []model.PluginMeta{createDefaultCheEditorMetaWithApiVersion("v100", "id3")},
			},
			want: want{
				err: "Plugin 'id3' is invalid. Field 'apiVersion' contains invalid version 'v100'",
			},
		},
		{
			name: "Sorts metas by type",
			//mocks: mocks{},
			args: args{
				metas: []model.PluginMeta{
					createVSCodeMeta("id1"),
					createChePluginMeta("id2"),
					createVSCodeMeta("id4"),
					createTheiaMeta("id3"),
					createTheiaMeta("id5"),
					createChePluginMeta("id6"),
				},
			},
			want: want{
				commonPlugins: []model.ChePlugin{createChePlugin("id2"), createChePlugin("id6")},
				vscodeMetas:   []model.PluginMeta{createVSCodeMeta("id1"), createVSCodeMeta("id4"), createTheiaMeta("id3"), createTheiaMeta("id5")},
			},
		},
		{
			name:  "Processes metas of type Che Editor with those of type Che Plugin",
			mocks: mocks{},
			args: args{
				metas: []model.PluginMeta{createChePluginMeta("id1"), createCheEditorMeta("id2")},
			},
			want: want{
				commonPlugins: []model.ChePlugin{createChePlugin("id1"), createChePlugin("id2")},
			},
		},
		{
			name:  "Properly converts PluginMeta to ChePlugin",
			mocks: mocks{},
			args: args{
				metas: []model.PluginMeta{
					{
						APIVersion:  "v2",
						Publisher:   "pub1",
						Name:        "name1",
						Version:     "v0.13",
						ID:          "id1",
						Type:        ChePluginType,
						Title:       "test title",
						DisplayName: "test display name",
						Description: "test description",
						Icon:        "https://icon.com/icon.svg",
						Spec: model.PluginMetaSpec{
							Endpoints: []model.Endpoint{
								{
									Name:       "end1",
									TargetPort: 80,
									Public:     true,
									Attributes: map[string]string{
										"attr1":     "val1",
										"testAttr2": "value2",
									},
								},
							},
							Containers: []model.Container{
								createContainer("container1"),
								createContainer("container2"),
							},
							WorkspaceEnv: []model.EnvVar{
								{
									Name:  "workspaceEnv1",
									Value: "something",
								},
								{
									Name:  "workspaceEnv2",
									Value: "somethingElse",
								},
							},
						},
					},
					{
						APIVersion:  "v2",
						Publisher:   "pub2",
						Name:        "name2",
						Version:     "v0",
						ID:          "id2",
						Type:        EditorPluginType,
						Title:       "test title",
						DisplayName: "test display name",
						Description: "test description",
						Icon:        "https://icon.com/icon.svg",
						Spec: model.PluginMetaSpec{
							Endpoints: []model.Endpoint{
								{
									Name:       "end2",
									TargetPort: 8080,
									Public:     false,
								},
							},
							Containers: []model.Container{
								createContainer("container3"),
							},
							WorkspaceEnv: []model.EnvVar{
								{
									Name:  "workspaceEnv3",
									Value: "something3",
								},
							},
						},
					}},
			},
			want: want{
				commonPlugins: []model.ChePlugin{
					{
						Publisher: "pub1",
						Name:      "name1",
						Version:   "v0.13",
						ID:        "id1",
						Endpoints: []model.Endpoint{
							{
								Name:       "end1",
								TargetPort: 80,
								Public:     true,
								Attributes: map[string]string{
									"attr1":     "val1",
									"testAttr2": "value2",
								},
							},
						},
						Containers: []model.Container{
							createContainer("container1"),
							createContainer("container2"),
						},
						WorkspaceEnv: []model.EnvVar{
							{
								Name:  "workspaceEnv1",
								Value: "something",
							},
							{
								Name:  "workspaceEnv2",
								Value: "somethingElse",
							},
						},
					},
					{
						Publisher: "pub2",
						Name:      "name2",
						Version:   "v0",
						ID:        "id2",
						Endpoints: []model.Endpoint{
							{
								Name:       "end2",
								TargetPort: 8080,
								Public:     false,
							},
						},
						Containers: []model.Container{
							createContainer("container3"),
						},
						WorkspaceEnv: []model.EnvVar{
							{
								Name:  "workspaceEnv3",
								Value: "something3",
							},
						},
					}},
			},
		},
		{
			name: "Meta type checking is case insensitive",
			args: args{
				metas: []model.PluginMeta{
					{
						Type:       "che plugin",
						ID:         "id11",
						APIVersion: "v2",
						Spec: model.PluginMetaSpec{
							Containers: []model.Container{
								{
									Image: defaultImage,
								},
							},
						},
					},
					{
						Type:       "Che Plugin",
						ID:         "id12",
						APIVersion: "v2",
						Spec: model.PluginMetaSpec{
							Containers: []model.Container{
								{
									Image: defaultImage,
								},
							},
						},
					},
					{
						Type:       "cHE plugIN",
						ID:         "id13",
						APIVersion: "v2",
						Spec: model.PluginMetaSpec{
							Containers: []model.Container{
								{
									Image: defaultImage,
								},
							},
						},
					},
					{
						Type:       "vs code extension",
						ID:         "id21",
						APIVersion: "v2",
						Spec: model.PluginMetaSpec{
							Extensions: []string{
								"some extensions is here",
							},
						},
					},
					{
						Type:       "VS CODE EXTENSION",
						ID:         "id22",
						APIVersion: "v2",
						Spec: model.PluginMetaSpec{
							Extensions: []string{
								"some extensions is here",
							},
						},
					},
					{
						Type:       "vs cODE EXTENSION",
						ID:         "id23",
						APIVersion: "v2",
						Spec: model.PluginMetaSpec{
							Extensions: []string{
								"some extensions is here",
							},
						},
					},
					{
						Type:       "theia plugin",
						ID:         "id31",
						APIVersion: "v2",
						Spec: model.PluginMetaSpec{
							Extensions: []string{
								"some extensions is here",
							},
						},
					},
					{
						Type:       "Theia Plugin",
						ID:         "id32",
						APIVersion: "v2",
						Spec: model.PluginMetaSpec{
							Extensions: []string{
								"some extensions is here",
							},
						},
					},
					{
						Type:       "THEIA PLUGIN",
						ID:         "id33",
						APIVersion: "v2",
						Spec: model.PluginMetaSpec{
							Extensions: []string{
								"some extensions is here",
							},
						},
					},
				},
			},
			want: want{
				vscodeMetas: []model.PluginMeta{
					{
						Type:       "vs code extension",
						ID:         "id21",
						APIVersion: "v2",
						Spec: model.PluginMetaSpec{
							Extensions: []string{
								"some extensions is here",
							},
						},
					},
					{
						Type:       "VS CODE EXTENSION",
						ID:         "id22",
						APIVersion: "v2",
						Spec: model.PluginMetaSpec{
							Extensions: []string{
								"some extensions is here",
							},
						},
					},
					{
						Type:       "vs cODE EXTENSION",
						ID:         "id23",
						APIVersion: "v2",
						Spec: model.PluginMetaSpec{
							Extensions: []string{
								"some extensions is here",
							},
						},
					},
					{
						Type:       "theia plugin",
						ID:         "id31",
						APIVersion: "v2",
						Spec: model.PluginMetaSpec{
							Extensions: []string{
								"some extensions is here",
							},
						},
					},
					{
						Type:       "Theia Plugin",
						ID:         "id32",
						APIVersion: "v2",
						Spec: model.PluginMetaSpec{
							Extensions: []string{
								"some extensions is here",
							},
						},
					},
					{
						Type:       "THEIA PLUGIN",
						ID:         "id33",
						APIVersion: "v2",
						Spec: model.PluginMetaSpec{
							Extensions: []string{
								"some extensions is here",
							},
						},
					},
				},
				commonPlugins: []model.ChePlugin{
					{
						ID: "id11",
						Containers: []model.Container{
							{
								Image: defaultImage,
							},
						},
					},
					{
						ID: "id12",
						Containers: []model.Container{
							{
								Image: defaultImage,
							},
						},
					},
					{
						ID: "id13",
						Containers: []model.Container{
							{
								Image: defaultImage,
							},
						},
					},
				},
			},
		},
		{
			name: "Returns error when type not supported",
			args: args{
				metas: []model.PluginMeta{
					{
						Type:       "Unsupported type",
						ID:         "test id",
						Version:    "test version",
						Publisher:  "test publisher",
						Name:       "test name",
						APIVersion: "v2",
					},
				},
			},
			want: want{
				err: "Type 'Unsupported type' of plugin 'test id' is unsupported",
			},
		},
		{
			name: "Returns error when type is empty",
			args: args{
				metas: []model.PluginMeta{
					{
						Type:       "",
						ID:         "test id",
						Version:    "test version",
						Publisher:  "test publisher",
						Name:       "test name",
						APIVersion: "v2",
					},
				},
			},
			want: want{
				err: "Type field is missing in meta information of plugin 'test id'",
			},
		},
		{
			name: "Returns error when storing common plugin fails",
			args: args{
				metas: []model.PluginMeta{
					{
						Type:       ChePluginType,
						ID:         "test id",
						Version:    "test version",
						Publisher:  "test publisher",
						Name:       "test name",
						APIVersion: "v2",
						Spec: model.PluginMetaSpec{
							Containers: []model.Container{
								{
									Image: defaultImage,
								},
							},
						},
					},
				},
			},
			mocks: mocks{
				storageError: errors.New("test error"),
			},
			want: want{
				err: "test error",
			},
		},
		{
			name: "Returns error when extensions field is specified in common che plugin",
			args: args{
				metas: []model.PluginMeta{{
					Type:       TestChePluginType,
					ID:         "test id",
					APIVersion: "v2",
					Spec: model.PluginMetaSpec{
						Extensions: []string{
							"some extensions is here",
						},
					},
				}},
			},
			want: want{
				err: "Plugin 'test id' is invalid. Field 'spec.extensions' is not allowed in plugin of type 'Che Plugin'",
			},
		},
		{
			name: "Returns error when extensions field is specified in che editor",
			args: args{
				metas: []model.PluginMeta{{
					Type:       TestEditorPluginType,
					ID:         "test id",
					APIVersion: "v2",
					Spec: model.PluginMetaSpec{
						Extensions: []string{
							"some extensions is here",
						},
					},
				}},
			},
			want: want{
				err: "Plugin 'test id' is invalid. Field 'spec.extensions' is not allowed in plugin of type 'Che Editor'",
			},
		},
		{
			name: "Returns error when extensions list is empty in vs code extension",
			args: args{
				metas: []model.PluginMeta{{
					Type:       TestVscodePluginType,
					ID:         "test id",
					APIVersion: "v2",
					Spec: model.PluginMetaSpec{
						Extensions: []string{
						},
					},
				}},
			},
			want: want{
				err: "Plugin 'test id' is invalid. Field 'spec.extensions' must not be empty",
			},
		},
		{
			name: "Returns error when extensions list is empty in theia plugin",
			args: args{
				metas: []model.PluginMeta{{
					Type:       TestTheiaPluginType,
					ID:         "test id",
					APIVersion: "v2",
					Spec: model.PluginMetaSpec{
						Extensions: []string{
						},
					},
				}},
			},
			want: want{
				err: "Plugin 'test id' is invalid. Field 'spec.extensions' must not be empty",
			},
		},
		{
			name: "Returns error when containers field is empty in common che plugin",
			args: args{
				metas: []model.PluginMeta{{
					Type:       TestChePluginType,
					ID:         "test id",
					APIVersion: "v2",
					Spec: model.PluginMetaSpec{
						Containers: []model.Container{},
					},
				}},
			},
			want: want{
				err: "Plugin 'test id' is invalid. Field 'spec.containers' must not be empty",
			},
		},
		{
			name: "Returns error when containers field is empty in editor",
			args: args{
				metas: []model.PluginMeta{{
					Type:       TestEditorPluginType,
					ID:         "test id",
					APIVersion: "v2",
					Spec: model.PluginMetaSpec{
						Containers: []model.Container{},
					},
				}},
			},
			want: want{
				err: "Plugin 'test id' is invalid. Field 'spec.containers' must not be empty",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createMocks()

			m.vscodeBroker.On("ProcessPlugin", mock.AnythingOfType("model.PluginMeta")).Return(tt.mocks.vsCodeError)
			m.storage.On("AddPlugin", mock.AnythingOfType("model.ChePlugin")).Return(tt.mocks.storageError)

			err := m.b.ProcessPlugins(tt.args.metas)

			if err != nil || tt.want.err != "" {
				assert.EqualError(t, err, tt.want.err)
			} else if tt.want.commonPlugins == nil && tt.want.vscodeMetas == nil {
				assert.Fail(t, "Neither expected error nor expected ProcessPlugin method arguments are set in test")
			}
			if tt.want.commonPlugins != nil {
				for _, plugin := range tt.want.commonPlugins {
					m.storage.AssertCalled(t, "AddPlugin", plugin)
				}
			}
			if tt.want.vscodeMetas != nil {
				for _, meta := range tt.want.vscodeMetas {
					m.vscodeBroker.AssertCalled(t, "ProcessPlugin", meta)
				}
			}
		})
	}
}

func TestBroker_getPluginMetas(t *testing.T) {
	const defaultRegistry = "http://defaultRegistry.com"
	const RegistryURLFormat = "%s/%s/meta.yaml"

	type args struct {
		fqns            []model.PluginFQN
		defaultRegistry string
	}
	type want struct {
		errRegexp *regexp.Regexp
		fetchURL  string
	}
	type mocks struct {
		fetchData  []byte
		fetchError error
	}
	successMock := mocks{
		fetchData:  []byte(""),
		fetchError: nil,
	}
	errorMock := mocks{
		fetchData:  nil,
		fetchError: errors.New("Test error"),
	}

	tests := []struct {
		name  string
		args  args
		mocks mocks
		want  want
	}{
		{
			name: "Returns error when unable to get registry",
			args: args{
				fqns:            []model.PluginFQN{pluginFQNWithoutRegistry},
				defaultRegistry: "",
			},
			want: want{
				errRegexp: regexp.MustCompile("plugin .* does not specify registry and no default is provided"),
			},
			mocks: successMock,
		},
		{
			name: "Uses default registry for plugins with no registry defined",
			args: args{
				fqns:            []model.PluginFQN{pluginFQNWithoutRegistry},
				defaultRegistry: defaultRegistry,
			},
			want: want{
				errRegexp: nil,
				fetchURL: fmt.Sprintf(
					RegistryURLFormat,
					defaultRegistry+"/plugins",
					pluginFQNWithoutRegistry.ID),
			},
			mocks: successMock,
		},
		{
			name: "Uses specified registry for plugins that define one",
			args: args{
				fqns:            []model.PluginFQN{pluginFQNWithRegistry},
				defaultRegistry: defaultRegistry,
			},
			want: want{
				errRegexp: nil,
				fetchURL: fmt.Sprintf(
					RegistryURLFormat,
					pluginFQNWithRegistry.Registry,
					pluginFQNWithRegistry.ID),
			},
			mocks: successMock,
		},
		{
			name: "Should not return error when all plugins specify registry",
			args: args{
				fqns:            []model.PluginFQN{pluginFQNWithRegistry},
				defaultRegistry: "",
			},
			want: want{
				errRegexp: nil,
				fetchURL: fmt.Sprintf(
					RegistryURLFormat,
					pluginFQNWithRegistry.Registry,
					pluginFQNWithRegistry.ID),
			},
			mocks: successMock,
		},
		{
			name: "Returns error when unable to get meta.yaml from registry",
			args: args{
				fqns:            []model.PluginFQN{pluginFQNWithoutRegistry},
				defaultRegistry: defaultRegistry,
			},
			want: want{
				errRegexp: regexp.MustCompile("failed to fetch plugin meta.yaml for plugin .* from registry .*"),
				fetchURL:  "",
			},
			mocks: errorMock,
		},
		{
			name: "Accounts for trailing slash in plugin registry field",
			args: args{
				fqns:            []model.PluginFQN{pluginFQNWithRegistryTrailingSlash},
				defaultRegistry: defaultRegistry,
			},
			want: want{
				errRegexp: nil,
				fetchURL: fmt.Sprintf(
					RegistryURLFormat,
					strings.TrimSuffix(pluginFQNWithRegistryTrailingSlash.Registry, "/"),
					pluginFQNWithRegistryTrailingSlash.ID),
			},
			mocks: successMock,
		},
		{
			name: "Accounts for trailing slash in default registry address",
			args: args{
				fqns:            []model.PluginFQN{pluginFQNWithoutRegistry},
				defaultRegistry: defaultRegistry + "/",
			},
			want: want{
				errRegexp: nil,
				fetchURL: fmt.Sprintf(
					RegistryURLFormat,
					defaultRegistry+"/plugins",
					pluginFQNWithoutRegistry.ID),
			},
			mocks: successMock,
		},
		{
			name: "Supports default registry address with path with trailing slash",
			args: args{
				fqns:            []model.PluginFQN{pluginFQNWithoutRegistry},
				defaultRegistry: defaultRegistry + "/v2/",
			},
			want: want{
				errRegexp: nil,
				fetchURL: fmt.Sprintf(
					RegistryURLFormat,
					defaultRegistry+"/v2/plugins",
					pluginFQNWithoutRegistry.ID),
			},
			mocks: successMock,
		},
		{
			name: "Supports default registry address with path with no trailing slash",
			args: args{
				fqns:            []model.PluginFQN{pluginFQNWithoutRegistry},
				defaultRegistry: defaultRegistry + "/v2",
			},
			want: want{
				errRegexp: nil,
				fetchURL: fmt.Sprintf(
					RegistryURLFormat,
					defaultRegistry+"/v2/plugins",
					pluginFQNWithoutRegistry.ID),
			},
			mocks: successMock,
		},
		{
			name: "Supports custom registry address with path with trailing slash",
			args: args{
				fqns: []model.PluginFQN{
					{
						ID:       "test-with-registry/2.0",
						Registry: "http://test-registry.com/v3/",
					}},
				defaultRegistry: defaultRegistry,
			},
			want: want{
				errRegexp: nil,
				fetchURL: fmt.Sprintf(
					RegistryURLFormat,
					"http://test-registry.com/v3",
					"test-with-registry/2.0"),
			},
			mocks: successMock,
		},
		{
			name: "Supports custom registry address with path with no trailing slash",
			args: args{
				fqns: []model.PluginFQN{
					{
						ID:       "test-with-registry/2.0",
						Registry: "http://test-registry.com/v4",
					}},
				defaultRegistry: defaultRegistry,
			},
			want: want{
				errRegexp: nil,
				fetchURL: fmt.Sprintf(
					RegistryURLFormat,
					"http://test-registry.com/v4",
					"test-with-registry/2.0"),
			},
			mocks: successMock,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createMocks()
			m.u.On("Fetch", mock.AnythingOfType("string")).Return(tt.mocks.fetchData, tt.mocks.fetchError)

			_, err := m.b.getPluginMetas(tt.args.fqns, tt.args.defaultRegistry)
			if tt.want.errRegexp != nil {
				assertErrorMatches(t, tt.want.errRegexp, err)
				return
			}
			assert.NoError(t, err)
			m.u.AssertCalled(t, "Fetch", tt.want.fetchURL)
		})
	}
}

func createDefaultVSCodeMeta() model.PluginMeta {
	return createVSCodeMeta("test ID")
}

func createDefaultTheiaMeta() model.PluginMeta {
	return createTheiaMeta("test ID")
}

func createDefaultChePluginMeta() model.PluginMeta {
	return createChePluginMeta("test ID")
}

func createDefaultVSCodeMetaWithApiVersion(APIVersion string, ID string) model.PluginMeta {
	meta := createVSCodeMeta(ID)
	meta.APIVersion = APIVersion
	return meta
}

func createDefaultTheiaMetaWithApiVersion(APIVersion string, ID string) model.PluginMeta {
	meta := createTheiaMeta(ID)
	meta.APIVersion = APIVersion
	return meta
}

func createDefaultChePluginMetaWithApiVersion(APIVersion string, ID string) model.PluginMeta {
	meta := createChePluginMeta(ID)
	meta.APIVersion = APIVersion
	return meta
}

func createDefaultCheEditorMetaWithApiVersion(APIVersion string, ID string) model.PluginMeta {
	meta := createCheEditorMeta(ID)
	meta.APIVersion = APIVersion
	return meta
}

func createVSCodeMeta(ID string) model.PluginMeta {
	return model.PluginMeta{
		Type:       TestVscodePluginType,
		ID:         ID,
		APIVersion: "v2",
		Spec: model.PluginMetaSpec{
			Extensions: []string{
				"some extensions is here",
			},
		},
	}
}

func createTheiaMeta(ID string) model.PluginMeta {
	return model.PluginMeta{
		Type:       TestTheiaPluginType,
		ID:         ID,
		APIVersion: "v2",
		Spec: model.PluginMetaSpec{
			Extensions: []string{
				"some extensions is here",
			},
		},
	}
}

func createChePluginMeta(ID string) model.PluginMeta {
	return model.PluginMeta{
		Type:       TestChePluginType,
		ID:         ID,
		APIVersion: "v2",
		Spec: model.PluginMetaSpec{
			Containers: []model.Container{
				{
					Image: defaultImage,
				},
			},
		},
	}
}

func createCheEditorMeta(ID string) model.PluginMeta {
	return model.PluginMeta{
		Type:       TestEditorPluginType,
		ID:         ID,
		APIVersion: "v2",
		Spec: model.PluginMetaSpec{
			Containers: []model.Container{
				{
					Image: defaultImage,
				},
			},
		},
	}
}

func createChePlugin(ID string) model.ChePlugin {
	return model.ChePlugin{
		ID: ID,
		Containers: []model.Container{
			{
				Image: defaultImage,
			},
		},
	}
}

func createContainer(name string) model.Container {
	return model.Container{
		Name:         name,
		MountSources: true,
		Volumes: []model.Volume{
			{
				Name:      "volume1",
				MountPath: "/some/where",
			},
		},
		Env: []model.EnvVar{
			{
				Name:  "env1",
				Value: "value1",
			},
		},
		Image: "testRegistry.com/user/repo:latest",
		Ports: []model.ExposedPort{
			{
				ExposedPort: 10000,
			},
		},
		MemoryLimit: "100GB",
		Commands: []model.Command{
			{
				Name:       "command1",
				Command:    []string{"tail", "-f", "/dev/null"},
				WorkingDir: "/plugins",
			},
		},
	}
}

var pluginFQNWithoutRegistry = model.PluginFQN{
	ID: "test-no-registry/1.0",
}

var pluginFQNWithRegistry = model.PluginFQN{
	ID:       "test-with-registry/2.0",
	Registry: "test-registry",
}

var pluginFQNWithRegistryTrailingSlash = model.PluginFQN{
	ID:       "test-with-registry-suffix/3.0",
	Registry: "test-registry/",
}

func assertErrorMatches(t *testing.T, expected *regexp.Regexp, actual error) {
	if actual == nil {
		t.Errorf("Expected error %s but got nil", expected.String())
	}
	if !expected.MatchString(actual.Error()) {
		t.Errorf("Error message does not match. Expected '%s' but got '%s'", expected.String(), actual.Error())
	}
}
