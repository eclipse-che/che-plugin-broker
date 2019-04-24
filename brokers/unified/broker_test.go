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

	cheBrokerMocks "github.com/eclipse/che-plugin-broker/brokers/che-plugin-broker/mocks"
	theiaBrokerMocks "github.com/eclipse/che-plugin-broker/brokers/theia/mocks"
	vscodeBrokerMocks "github.com/eclipse/che-plugin-broker/brokers/vscode/mocks"
	cmock "github.com/eclipse/che-plugin-broker/common/mocks"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
	fmock "github.com/eclipse/che-plugin-broker/utils/mocks"
	"github.com/stretchr/testify/mock"
)

const TestChePluginType = "Che Plugin"
const TestEditorPluginType = "Che Editor"
const TestTheiaPluginType = "Theia plugin"
const TestVscodePluginType = "VS Code extension"

type mocks struct {
	cb           *cmock.Broker
	u            *fmock.IoUtil
	randMock     *cmock.Random
	b            *Broker
	theiaBroker  *theiaBrokerMocks.Broker
	vscodeBroker *vscodeBrokerMocks.Broker
	cheBroker    *cheBrokerMocks.ChePluginBroker
}

func createMocks() *mocks {
	cb := &cmock.Broker{}
	u := &fmock.IoUtil{}
	randMock := &cmock.Random{}
	theiaBroker := &theiaBrokerMocks.Broker{}
	vscodeBroker := &vscodeBrokerMocks.Broker{}
	cheBroker := &cheBrokerMocks.ChePluginBroker{}

	cb.On("PrintInfo", mock.AnythingOfType("string"))

	return &mocks{
		cb:       cb,
		u:        u,
		randMock: randMock,
		b: &Broker{
			Broker:  cb,
			Storage: storage.New(),
			utils:   u,

			theiaBroker:  theiaBroker,
			vscodeBroker: vscodeBroker,
			cheBroker:    cheBroker,
		},
		theiaBroker:  theiaBroker,
		vscodeBroker: vscodeBroker,
		cheBroker:    cheBroker,
	}
}

func TestBroker_processPlugins(t *testing.T) {
	type args struct {
		metas []model.PluginMeta
	}
	type want struct {
		err            string
		theiaMetas     []model.PluginMeta
		vscodeMetas    []model.PluginMeta
		cheBrokerMetas []model.PluginMeta
	}
	type mocks struct {
		vsCodeError    error
		theiaError     error
		cheBrokerError error
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
			name: "Returns error on Theia broker error",
			mocks: mocks{
				theiaError: errors.New("test theia error"),
			},
			args: args{
				metas: []model.PluginMeta{createDefaultVSCodeMeta(), createDefaultTheiaMeta(), createDefaultChePluginMeta()},
			},
			want: want{
				err: "test theia error",
			},
		},
		{
			name: "Returns error on Che plugin broker error",
			mocks: mocks{
				theiaError: errors.New("test che plugin broker error"),
			},
			args: args{
				metas: []model.PluginMeta{createDefaultVSCodeMeta(), createDefaultChePluginMeta(), createDefaultTheiaMeta()},
			},
			want: want{
				err: "test che plugin broker error",
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
				cheBrokerMetas: []model.PluginMeta{createChePluginMeta("id2"), createChePluginMeta("id6")},
				vscodeMetas:    []model.PluginMeta{createVSCodeMeta("id1"), createVSCodeMeta("id4")},
				theiaMetas:     []model.PluginMeta{createTheiaMeta("id3"), createTheiaMeta("id5")},
			},
		},
		{
			name:  "Processes metas of type Che Editor with those of type Che Plugin",
			mocks: mocks{},
			args: args{
				metas: []model.PluginMeta{createChePluginMeta("id1"), createCheEditorMeta("id2")},
			},
			want: want{
				cheBrokerMetas: []model.PluginMeta{createChePluginMeta("id1"), createCheEditorMeta("id2")},
			},
		},
		{
			name: "Meta type checking is case insensitive",
			args: args{
				metas: []model.PluginMeta{
					{
						Type: "che plugin",
						ID:   "id11",
					},
					{
						Type: "Che Plugin",
						ID:   "id12",
					},
					{
						Type: "cHE plugIN",
						ID:   "id13",
					},
					{
						Type: "vs code extension",
						ID:   "id21",
					},
					{
						Type: "VS CODE EXTENSION",
						ID:   "id22",
					},
					{
						Type: "vs cODE EXTENSION",
						ID:   "id23",
					},
					{
						Type: "theia plugin",
						ID:   "id31",
					},
					{
						Type: "Theia Plugin",
						ID:   "id32",
					},
					{
						Type: "THEIA PLUGIN",
						ID:   "id33",
					},
				},
			},
			want: want{
				theiaMetas: []model.PluginMeta{
					{
						Type: "theia plugin",
						ID:   "id31",
					},
					{
						Type: "Theia Plugin",
						ID:   "id32",
					},
					{
						Type: "THEIA PLUGIN",
						ID:   "id33",
					},
				},
				vscodeMetas: []model.PluginMeta{
					{
						Type: "vs code extension",
						ID:   "id21",
					},
					{
						Type: "VS CODE EXTENSION",
						ID:   "id22",
					},
					{
						Type: "vs cODE EXTENSION",
						ID:   "id23",
					},
				},
				cheBrokerMetas: []model.PluginMeta{
					{
						Type: "che plugin",
						ID:   "id11",
					},
					{
						Type: "Che Plugin",
						ID:   "id12",
					},
					{
						Type: "cHE plugIN",
						ID:   "id13",
					},
				},
			},
		},
		{
			name: "Returns error when type not supported",
			args: args{
				metas: []model.PluginMeta{
					{
						Type:    "Unsupported type",
						ID:      "test id",
						Version: "test version",
						Publisher: "test publisher",
						Name: "test name",
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
						Type:    "",
						ID:      "test id",
						Version: "test version",
						Publisher: "test publisher",
						Name: "test name",
					},
				},
			},
			want: want{
				err: "Type field is missing in meta information of plugin 'test id'",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := createMocks()

			m.cheBroker.On("ProcessPlugin", mock.AnythingOfType("model.PluginMeta")).Return(tt.mocks.cheBrokerError)
			m.theiaBroker.On("ProcessPlugin", mock.AnythingOfType("model.PluginMeta")).Return(tt.mocks.theiaError)
			m.vscodeBroker.On("ProcessPlugin", mock.AnythingOfType("model.PluginMeta")).Return(tt.mocks.vsCodeError)

			err := m.b.ProcessPlugins(tt.args.metas)

			if err != nil || tt.want.err != "" {
				assert.EqualError(t, err, tt.want.err)
			} else if tt.want.cheBrokerMetas == nil && tt.want.theiaMetas == nil && tt.want.vscodeMetas == nil {
				assert.Fail(t, "Neither expected error nor expected ProcessPlugin method arguments are set in test")
			}
			if tt.want.cheBrokerMetas != nil {
				for _, meta := range tt.want.cheBrokerMetas {
					m.cheBroker.AssertCalled(t, "ProcessPlugin", meta)
				}
			}
			if tt.want.theiaMetas != nil {
				for _, meta := range tt.want.theiaMetas {
					m.theiaBroker.AssertCalled(t, "ProcessPlugin", meta)
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

func createVSCodeMeta(ID string) model.PluginMeta {
	return model.PluginMeta{
		Type: TestVscodePluginType,
		ID:   ID,
	}
}

func createTheiaMeta(ID string) model.PluginMeta {
	return model.PluginMeta{
		Type: TestTheiaPluginType,
		ID:   ID,
	}
}

func createChePluginMeta(ID string) model.PluginMeta {
	return model.PluginMeta{
		Type: TestChePluginType,
		ID:   ID,
	}
}

func createCheEditorMeta(ID string) model.PluginMeta {
	return model.PluginMeta{
		Type: TestEditorPluginType,
		ID:   ID,
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
