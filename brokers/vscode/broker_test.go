//
// Copyright (c) 2018-2019 Red Hat, Inc.
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
	"bytes"
	"errors"
	"fmt"
	"github.com/eclipse/che-plugin-broker/utils"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/eclipse/che-plugin-broker/brokers/test"
	tests "github.com/eclipse/che-plugin-broker/brokers/test"
	"github.com/eclipse/che-plugin-broker/common"
	cmock "github.com/eclipse/che-plugin-broker/common/mocks"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
	fmock "github.com/eclipse/che-plugin-broker/utils/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	extName       = "Test-name"
	extPublisher  = "Test-publisher"
	vsixURL       = "http://test.url"
	vsixBrokenURL = "http://broken.test.url"
	pluginID      = "tid"
	pluginVersion = "tv"
	image         = "test/test:tag"
)

type mocks struct {
	cb       *cmock.Broker
	u        *fmock.IoUtil
	b        *Broker
	randMock *cmock.Random
}

func initMocks() *mocks {
	cb := &cmock.Broker{}
	u := &fmock.IoUtil{}
	randMock := &cmock.Random{}
	return &mocks{
		cb:       cb,
		u:        u,
		randMock: randMock,
		b: &Broker{
			Broker:  cb,
			ioUtil:  u,
			Storage: storage.New(),
			client:  test.NewTestHTTPClient(okMarketplaceResponse),
			rand:    randMock,
		},
	}
}

func TestStart(t *testing.T) {
	m := initMocks()

	m.cb.On("PubStarted").Once()
	m.cb.On("PrintDebug", mock.AnythingOfType("string"))
	m.cb.On("PubDone", "null").Once()
	m.cb.On("PrintInfo", mock.AnythingOfType("string"))
	m.cb.On("PrintPlan", mock.AnythingOfType("[]model.PluginMeta")).Once()
	m.cb.On("CloseConsumers").Once()

	m.b.Start([]model.PluginMeta{})

	m.cb.AssertExpectations(t)
}

func TestProcessPluginBrokenUrl(t *testing.T) {
	m := initMocks()
	meta := model.PluginMeta{
		ID:      pluginID,
		Version: pluginVersion,
		URL:     vsixBrokenURL,
		Attributes: map[string]string{
			"containerImage": image,
		},
	}

	workDir := tests.CreateTestWorkDir()

	setUpDownloadFailureCase(workDir, m)
	defer tests.RemoveAll(workDir)
	err := m.b.ProcessPlugin(meta)

	assert.NotNil(t, err)
	assert.EqualError(t, err, "Failed to download plugin")

	pluginsPointer, err := m.b.Storage.Plugins()
	assert.Nil(t, err)
	assert.NotNil(t, pluginsPointer)
	plugins := *pluginsPointer
	assert.Equal(t, 0, len(plugins))

	m.cb.AssertExpectations(t)
	m.u.AssertExpectations(t)

	m.u.AssertNotCalled(t, "Unzip", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.u.AssertNotCalled(t, "CopyResource", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
}

func TestBroker_processPlugin(t *testing.T) {
	cases := []struct {
		name      string
		meta      model.PluginMeta
		err       string
		want      []model.ChePlugin
		unzipFunc UnzipFunc
	}{
		{
			name: "Return error when neither extension nor URL nor extensions are present",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				Attributes: map[string]string{
					"containerImage": image,
				},
			},
			err: fmt.Sprintf(errorNoExtFieldsTemplate, "tid", "tv"),
		},
		{
			name: "Return error when neither attributes nor URL nor extensions are present",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
			},
			err: fmt.Sprintf(errorNoExtFieldsTemplate, "tid", "tv"),
		},
		{
			name: "Return error when neither attributes nor URL are present and extensions are nil",
			meta: model.PluginMeta{
				ID:         pluginID,
				Version:    pluginVersion,
				Extensions: nil,
			},
			err: fmt.Sprintf(errorNoExtFieldsTemplate, "tid", "tv"),
		},
		{
			name: "Return error when neither attributes nor URL are present and extensions are empty",
			meta: model.PluginMeta{
				ID:         pluginID,
				Version:    pluginVersion,
				Extensions: []string{},
			},
			err: fmt.Sprintf(errorNoExtFieldsTemplate, "tid", "tv"),
		},
		{
			name: "Return error when both extension and URL fields are present",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				URL:     vsixURL,
				Attributes: map[string]string{
					"extension":      "vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
					"containerImage": image,
				},
			},
			err: fmt.Sprintf(errorMutuallyExclusiveExtFieldsTemplate, "tid", "tv"),
		},
		{
			name: "Return error when both extensions and URL fields are present",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				URL:     vsixURL,
				Extensions: []string{
					"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
				},
			},
			err: fmt.Sprintf(errorMutuallyExclusiveExtFieldsTemplate, "tid", "tv"),
		},
		{
			name: "Return error when both extensions and extension fields are present",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				Extensions: []string{
					"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
				},
				Attributes: map[string]string{
					"extension": "vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
				},
			},
			err: fmt.Sprintf(errorMutuallyExclusiveExtFieldsTemplate, "tid", "tv"),
		},
		{
			name: "Successful brokering of remote plugin with extension field",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				Attributes: map[string]string{
					"extension":      "vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
					"containerImage": image,
				},
			},
			want: expectedPluginsWithSingleRemotePlugin(),
		},
		{
			name: "Successful brokering of remote plugin with URL field",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				URL:     vsixURL,
				Attributes: map[string]string{
					"containerImage": image,
				},
			},
			want: expectedPluginsWithSingleRemotePlugin(),
		},
		{
			name: "Successful brokering of local plugin with extension field",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				Attributes: map[string]string{
					"extension": "vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
				},
			},
			want: expectedPluginsWithSingleLocalPlugin(),
		},
		{
			name: "Successful brokering of local plugin with URL field and empty attributes",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				URL:     vsixURL,
				Attributes: map[string]string{
				},
			},
			want: expectedPluginsWithSingleLocalPlugin(),
		},
		{
			name: "Successful brokering of local plugin with URL field",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				URL:     vsixURL,
			},
			want: expectedPluginsWithSingleLocalPlugin(),
		},
		{
			name: "Successful brokering of local plugin with extensions field",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				Extensions: []string{
					"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
				},
			},
			want: expectedPluginsWithSingleLocalPlugin(),
		},
		{
			name: "Successful brokering of local plugin with extensions field and empty attributes",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				Attributes: map[string]string{
				},
				Extensions: []string{
					"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
				},
			},
			want: expectedPluginsWithSingleLocalPlugin(),
		},
		{
			name: "Successful brokering of local plugin with extensions field with several extensions",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				Extensions: []string{
					"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
					"vscode:extension/redhat-com.vscode-jdt-ls",
					"vscode:extension/redhat-com.vscode-maven",
				},
			},
			want: expectedPluginsWithSingleLocalPlugin(),
		},
		{
			name: "Successful brokering of local plugin with extensions field with mixed extensions and archives URLs",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				Extensions: []string{
					"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
					"vscode:extension/redhat-com.vscode-jdt-ls",
					"vscode:extension/redhat-com.vscode-maven",
				},
			},
			want: expectedPluginsWithSingleLocalPlugin(),
		},
		{
			name: "Successful brokering of remote plugin with extensions field with several extensions",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				Extensions: []string{
					"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
					"vscode:extension/redhat-com.vscode-jdt-ls",
					"vscode:extension/redhat-com.vscode-maven",
				},
				Attributes: map[string]string{
					"containerImage": image,
				},
			},
			unzipFunc: createUnzipFuncStub(
				generatePackageJSON("ms-kubernetes-tools", "vscode-kubernetes-tools"),
				generatePackageJSON("redhat-com", "vscode-jdt-ls"),
				generatePackageJSON("redhat-com", "vscode-maven")),
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				generateTheiaEnvVar("ms_kubernetes_tools_vscode_kubernetes_tools"),
				generateTheiaEnvVar("redhat_com_vscode_jdt_ls"),
				generateTheiaEnvVar("redhat_com_vscode_maven")),
		},
		{
			name: "Successful brokering of remote plugin with extensions field with mixed extensions and archives URLs",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				Extensions: []string{
					"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
					vsixURL,
					"vscode:extension/redhat-com.vscode-maven",
				},
				Attributes: map[string]string{
					"containerImage": image,
				},
			},
			unzipFunc: createUnzipFuncStub(
				generatePackageJSON("ms-kubernetes-tools", "vscode-kubernetes-tools"),
				generatePackageJSON("redhat-com", "vscode-jdt-ls"),
				generatePackageJSON("redhat-com", "vscode-maven")),
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				generateTheiaEnvVar("ms_kubernetes_tools_vscode_kubernetes_tools"),
				generateTheiaEnvVar("redhat_com_vscode_jdt_ls"),
				generateTheiaEnvVar("redhat_com_vscode_maven")),
		},
		{
			name: "Successful brokering of remote plugin with extensions field",
			meta: model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				Extensions: []string{
					"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
				},
				Attributes: map[string]string{
					"containerImage": image,
				},
			},
			unzipFunc: createUnzipFuncStub(
				generatePackageJSON("ms-kubernetes-tools", "vscode-kubernetes-tools")),
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				generateTheiaEnvVar("ms_kubernetes_tools_vscode_kubernetes_tools")),
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			m := initMocks()
			workDir := tests.CreateTestWorkDir()
			defer tests.RemoveAll(workDir)
			setUpSuccessfulCase(workDir, tt.meta, m, tt.unzipFunc)

			if tt.want == nil && tt.err == "" {
				t.Fatal("Neither want nor error are defined")
			}
			err := m.b.ProcessPlugin(tt.meta)
			if err != nil {
				if tt.err != "" {
					assert.EqualError(t, err, tt.err)
				} else {
					t.Errorf("ProcessPlugin() error = %v, wanted error %v", err, tt.err)
					return
				}
			} else {
				pluginsPointer, err := m.b.Storage.Plugins()
				assert.Nil(t, err)
				assert.NotNil(t, pluginsPointer)
				plugins := *pluginsPointer
				assert.Equal(t, tt.want, plugins)
			}
		})
	}
}

func generatePackageJSON(publisher string, name string) model.PackageJSON {
	return model.PackageJSON{
		Name:      name,
		Publisher: publisher,
	}
}

func generateTheiaEnvVar(prettyID string) string {
	return "THEIA_PLUGIN_REMOTE_ENDPOINT_" + prettyID
}

func expectedPluginsWithSingleRemotePluginWithSeveralExtensions(pluginTheiaEndpointVars ...string) []model.ChePlugin {
	expectedPlugin := model.ChePlugin{
		ID:      pluginID,
		Version: pluginVersion,
		Endpoints: []model.Endpoint{
			{
				Name:       "randomString1234567890",
				Public:     false,
				TargetPort: 4242,
			},
		},
		Containers: []model.Container{
			{
				Name:  "pluginsidecarrandomString123456",
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
				Ports: []model.ExposedPort{
					{
						ExposedPort: 4242,
					},
				},
				Env: []model.EnvVar{
					{
						Name:  "THEIA_PLUGIN_ENDPOINT_PORT",
						Value: "4242",
					},
				},
			},
		},
	}
	for _, envVarName := range pluginTheiaEndpointVars {
		expectedPlugin.WorkspaceEnv = append(expectedPlugin.WorkspaceEnv, model.EnvVar{
			Name:  envVarName,
			Value: "ws://randomString1234567890:4242",
		})
	}

	return []model.ChePlugin{
		expectedPlugin,
	}
}

func expectedPluginsWithSingleRemotePlugin() []model.ChePlugin {
	prettyID := "Test_publisher_Test_name"
	expectedPlugins := []model.ChePlugin{
		{
			ID:      pluginID,
			Version: pluginVersion,
			Endpoints: []model.Endpoint{
				{
					Name:       "randomString1234567890",
					Public:     false,
					TargetPort: 4242,
				},
			},
			Containers: []model.Container{
				{
					Name:  "pluginsidecarrandomString123456",
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
					Ports: []model.ExposedPort{
						{
							ExposedPort: 4242,
						},
					},
					Env: []model.EnvVar{
						{
							Name:  "THEIA_PLUGIN_ENDPOINT_PORT",
							Value: "4242",
						},
					},
				},
			},
			WorkspaceEnv: []model.EnvVar{
				{
					Name:  "THEIA_PLUGIN_REMOTE_ENDPOINT_" + prettyID,
					Value: "ws://randomString1234567890:4242",
				},
			},
		},
	}
	return expectedPlugins
}

func expectedPluginsWithSingleLocalPlugin() []model.ChePlugin {
	expectedPlugins := []model.ChePlugin{
		{
			ID:      pluginID,
			Version: pluginVersion,
		},
	}
	return expectedPlugins
}

func setUpSuccessfulCase(workDir string, meta model.PluginMeta, m *mocks, unzipFunc UnzipFunc) {
	_unzipFunc := defaultUnzipFunc()
	if unzipFunc != nil {
		_unzipFunc = unzipFunc
	}
	m.u.On("Unzip", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Run(_unzipFunc).Return(nil)
	m.u.On("CopyResource", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
	pluginPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s.randomString1234567890.vsix", meta.ID, meta.Version))
	m.u.On("CopyFile", mock.AnythingOfType("string"), pluginPath).Return(nil)
	m.cb.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.cb.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.cb.On("PrintInfo", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.u.On("Download", vsixURL, mock.AnythingOfType("string")).Return(nil)
	m.u.On("TempDir", "", "vscode-extension-broker").Return(workDir, nil)
	m.randMock.On("IntFromRange", 4000, 10000).Return(4242)
	m.randMock.On("String", 10).Return("randomString1234567890")
	m.randMock.On("String", 6).Return("randomString123456")
}

type UnzipFunc func(args mock.Arguments)

func defaultUnzipFunc() UnzipFunc {
	return func(args mock.Arguments) {
		dest := args[1].(string)
		packageJSON := model.PackageJSON{
			Name:      extName,
			Publisher: extPublisher,
		}
		packageJSONParent := filepath.Join(dest, "extension")
		tests.CreateDirs(packageJSONParent)
		packageJSONPath := filepath.Join(packageJSONParent, "package.json")
		tests.CreateFileWithContent(packageJSONPath, tests.ToJSONQuiet(packageJSON))
	}
}

func createUnzipFuncStub(pjs ...model.PackageJSON) UnzipFunc {
	jsons := pjs
	return func(args mock.Arguments) {
		dest := args[1].(string)
		packageJSONParent := filepath.Join(dest, "extension")
		tests.CreateDirs(packageJSONParent)
		packageJSONPath := filepath.Join(packageJSONParent, "package.json")
		json := jsons[0]
		if len(jsons) > 1 {
			jsons = jsons[1:]
		}
		tests.CreateFileWithContent(packageJSONPath, tests.ToJSONQuiet(json))
	}
}

func setUpDownloadFailureCase(workDir string, m *mocks) {
	m.cb.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.cb.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.cb.On("PrintInfo", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.u.On("Download", vsixBrokenURL, mock.AnythingOfType("string")).Return(errors.New("Failed to download plugin")).Once()
	m.u.On("TempDir", "", "vscode-extension-broker").Return(workDir, nil).Once()
	m.randMock.On("IntFromRange", 4000, 10000).Return(4242).Once()
	m.randMock.On("String", 10).Return("randomString1234567890")
	m.randMock.On("String", 6).Return("randomString123456")
}

func TestFetchExtensionInfo(t *testing.T) {
	cases := []struct {
		want    []byte
		ext     string
		err     string
		roundTF test.RoundTripFunc
	}{
		{
			err: "Parsing of VS Code extension ID 'invalidExt' failed for plugin 'tid:tv'. Extension should start from 'vscode:extension/'",
			ext: "invalidExt",
		},
		{
			ext:  "vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
			want: []byte("OK"),
			roundTF: func(req *http.Request) *http.Response {
				return &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(bytes.NewBufferString(`OK`)),
				}
			},
		},
		{
			ext:  "vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
			want: []byte("OK"),
			roundTF: func(req *http.Request) *http.Response {
				if req.Header.Get("Accept") != "application/json;api-version=3.0-preview.1" {
					t.Error("Accept header is incorrect")
				}
				return &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(bytes.NewBufferString(`OK`)),
				}
			},
		},
		{
			ext:  "vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
			want: []byte("OK"),
			roundTF: func(req *http.Request) *http.Response {
				if req.Header.Get("Content-Type") != "application/json" {
					t.Error("Content-Type header is incorrect")
				}
				return &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(bytes.NewBufferString(`OK`)),
				}
			},
		},
		{
			ext: "vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
			err: "VS Code extension downloading failed tid:tv. Status: 400. Body: ",
			roundTF: func(req *http.Request) *http.Response {
				return &http.Response{
					StatusCode: 400,
					Body:       ioutil.NopCloser(bytes.NewBufferString("")),
				}
			},
		},
		{
			ext: "vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
			err: "VS Code extension downloading failed tid:tv. Status: 400. Body: " + "test error",
			roundTF: func(req *http.Request) *http.Response {
				return &http.Response{
					StatusCode: 400,
					Body:       ioutil.NopCloser(bytes.NewBufferString("test error")),
				}
			},
		},
	}
	for _, tt := range cases {
		t.Run("", func(t *testing.T) {
			if tt.want == nil && tt.err == "" {
				t.Fatal("Neither want nor error are defined")
			}
			var b = &Broker{
				Broker:  common.NewBroker(),
				ioUtil:  utils.New(),
				Storage: storage.New(),
				client:  test.NewTestHTTPClient(tt.roundTF),
				rand:    common.NewRand(),
			}
			got, err := b.fetchExtensionInfo(tt.ext, model.PluginMeta{
				ID:      "tid",
				Version: "tv",
			})
			if err != nil {
				if tt.err != "" {
					assert.EqualError(t, err, tt.err)
				} else {
					t.Errorf("fetchExtensionInfo() error = %v, wanted error %v", err, tt.err)
					return
				}
			} else {
				if !bytes.Equal(got, tt.want) {
					t.Errorf("fetchExtensionInfo() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestFindAssetURL(t *testing.T) {
	cases := []struct {
		response []byte
		want     string
		err      string
	}{
		{
			err:      "Failed to parse VS Code extension marketplace response for plugin tid:v",
			response: []byte("{"),
		},
		{
			err:      "Failed to parse VS Code extension marketplace response for plugin tid:v",
			response: []byte("{}"),
		},
		{
			err:      "Failed to parse VS Code extension marketplace response for plugin tid:v",
			response: []byte(`{"results":[]}`),
		},
		{
			err:      "Failed to parse VS Code extension marketplace response for plugin tid:v",
			response: []byte(`{"results":null}`),
		},
		{
			err: "Failed to parse VS Code extension marketplace response for plugin tid:v",
			response: []byte(
				`{
				     "results":[
						 {
							"extensions":[
								{
									"versions": [
										{
											"files":[
											]
										}
									]
								}
							]
						 }
					 ]
			     }`),
		},
		{
			err: "Failed to parse VS Code extension marketplace response for plugin tid:v",
			response: []byte(
				`{
				     "results":[
						 {
							"extensions":[
								{
									"versions": [
										{
											"files":null
										}
									]
								}
							]
						 }
					 ]
			     }`),
		},
		{
			err: "Failed to parse VS Code extension marketplace response for plugin tid:v",
			response: []byte(
				`{
				     "results":[
						 {
							"extensions":[
								{
									"versions": [
									]
								}
							]
						 }
					 ]
			     }`),
		},
		{
			err: "Failed to parse VS Code extension marketplace response for plugin tid:v",
			response: []byte(
				`{
				     "results":[
						 {
							"extensions":[
								{
									"versions": null
								}
							]
						 }
					 ]
			     }`),
		},
		{
			err: "Failed to parse VS Code extension marketplace response for plugin tid:v",
			response: []byte(
				`{
				     "results":[
						 {
							"extensions":[
							]
						 }
					 ]
			     }`),
		},
		{
			err: "Failed to parse VS Code extension marketplace response for plugin tid:v",
			response: []byte(
				`{
				     "results":[
						 {
							"extensions":null
						 }
					 ]
			     }`),
		},
		{
			err: "VS Code extension archive information is not found in marketplace response for plugin tid:v",
			response: []byte(
				`{
				     "results":[
						 {
							"extensions":[
								{
									"versions": [
										{
											"files":[
												{

												}
											]
										}
									]
								}
							]
						 }
					 ]
			     }`),
		},
		{
			want: "good source",
			response: []byte(
				`{
				     "results":[
						 {
							"extensions":[
								{
									"versions": [
										{
											"files":[
												{
													"assetType":"extension",
													"source":"bad source"
												},
												{
													"assetType":"Microsoft.VisualStudio.Services.VSIXPackage",
													"source":"good source"
												}
											]
										}
									]
								}
							]
						 }
					 ]
			     }`),
		},
	}
	for _, tt := range cases {
		t.Run("", func(t *testing.T) {
			if tt.want == "" && tt.err == "" {
				t.Fatal("Neither want nor error are defined")
			}
			got, err := findAssetURL(tt.response, model.PluginMeta{
				ID:      "tid",
				Version: "v",
			})
			if err != nil {
				if tt.err != "" {
					assert.EqualError(t, err, tt.err)
				} else {
					t.Errorf("findAssetURL() error = %v, wanted error %v", err, tt.err)
					return
				}
			} else {
				if got != tt.want {
					t.Errorf("findAssetURL() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func okMarketplaceResponse(req *http.Request) *http.Response {
	JSON := `{
		"results":[
			{
			   "extensions":[
				   {
					   "versions": [
						   {
							   "files":[
								   {
									   "assetType":"Microsoft.VisualStudio.Services.VSIXPackage",
									   "source": "` + vsixURL + `"
								   }
							   ]
						   }
					   ]
				   }
			   ]
			}
		]
	}`
	return &http.Response{
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewBufferString(JSON)),
	}
}
