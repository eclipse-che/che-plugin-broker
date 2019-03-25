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
	"github.com/stretchr/testify/assert"
	"testing"

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
				theiaMetas:     []model.PluginMeta{createTheiaMeta("id3"), createTheiaMeta("id5"),},
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
					},
				},
			},
			want: want{
				err: "Type 'Unsupported type' of plugin 'test id:test version' is unsupported",
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
					},
				},
			},
			want: want{
				err: "Type field is missing in meta information of plugin 'test id:test version'",
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
