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
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/eclipse/che-plugin-broker/brokers/test"
	tests "github.com/eclipse/che-plugin-broker/brokers/test"
	"github.com/eclipse/che-plugin-broker/common"
	cmock "github.com/eclipse/che-plugin-broker/common/mocks"
	"github.com/eclipse/che-plugin-broker/files"
	fmock "github.com/eclipse/che-plugin-broker/files/mocks"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
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
	err := m.b.processPlugin(meta)

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
		name    string
		meta model.PluginMeta
		err string
		want []model.ChePlugin
	}{
		{
			name:"Return error when neither extension nor URL are present",
			meta:model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				Attributes: map[string]string{
					"containerImage": image,
				},
			},
			err:"Neither 'extension' no 'url' attributes found in VS Code extension description of the plugin tid:tv",
		},
		{
			name:"Return error when neither attributes nor URL are present",
			meta:model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
			},
			err:"Neither 'extension' no 'url' attributes found in VS Code extension description of the plugin tid:tv",
		},
		{
			name:"Return error when both extension and URL are present",
			meta:model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				URL:     vsixURL,
				Attributes: map[string]string{
					"extension": "vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
					"containerImage": image,
				},
			},
			err:"VS Code extension description of the plugin tid:tv might contain either 'extension' or 'url' attributes, but both of them are found",
		},
		{
			name:"Successful brokering of remote plugin with extension field",
			meta:model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				Attributes: map[string]string{
					"extension": "vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
					"containerImage": image,
				},
			},
			want:expectedPluginsWithSingleRemotePlugin(),
		},
		{
			name:"Successful brokering of remote plugin with URL field",
			meta:model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				URL:     vsixURL,
				Attributes: map[string]string{
					"containerImage": image,
				},
			},
			want:expectedPluginsWithSingleRemotePlugin(),
		},
		{
			name:"Successful brokering of local plugin with extension field",
			meta:model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				Attributes: map[string]string{
					"extension": "vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
				},
			},
			want:expectedPluginsWithSingleLocalPlugin(),
		},
		{
			name:"Successful brokering of local plugin with URL field",
			meta:model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				URL:     vsixURL,
				Attributes: map[string]string{
				},
			},
			want:expectedPluginsWithSingleLocalPlugin(),
		},
		{
			name:"Successful brokering of local plugin with URL field",
			meta:model.PluginMeta{
				ID:      pluginID,
				Version: pluginVersion,
				URL:     vsixURL,
			},
			want:expectedPluginsWithSingleLocalPlugin(),
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			m := initMocks()
			workDir := tests.CreateTestWorkDir()
			defer tests.RemoveAll(workDir)
			setUpSuccessfulCase(workDir, tt.meta, m)

			if tt.want == nil && tt.err == "" {
				t.Fatal("Neither want nor error are defined")
			}
			err := m.b.processPlugin(tt.meta)
			if err != nil {
				if tt.err != "" {
					assert.EqualError(t, err, tt.err)
				} else {
					t.Errorf("processPlugin() error = %v, wanted error %v", err, tt.err)
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

func expectedPluginsWithSingleRemotePlugin() []model.ChePlugin {
	prettyID := "Test_publisher_Test_name"
	expectedPlugins := []model.ChePlugin{
		{
			ID:      pluginID,
			Version: pluginVersion,
			Endpoints: []model.Endpoint{
				{
					Name:       "randomEndpointName",
					Public:     false,
					TargetPort: 4242,
				},
			},
			Containers: []model.Container{
				{
					Name:  "pluginsidecarrandomContainerSuffix",
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
					Value: "ws://randomEndpointName:4242",
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

func setUpSuccessfulCase(workDir string, meta model.PluginMeta, m *mocks) {
	archivePath := filepath.Join(workDir, "pluginArchive")
	unarchivedPath := filepath.Join(workDir, "plugin")
	packageJSONPath := filepath.Join(unarchivedPath, "extension", "package.json")
	pluginPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s", meta.ID, meta.Version))
	packageJSON := model.PackageJSON{
		Name:      extName,
		Publisher: extPublisher,
	}
	m.u.On("Unzip", archivePath, unarchivedPath).Return(func(archive string, dest string) error {
		tests.CreateDirs(filepath.Join(dest, "extension"))
		tests.CreateFileWithContent(packageJSONPath, tests.ToJSONQuiet(packageJSON))
		return nil
	})
	m.u.On("CopyResource", unarchivedPath, pluginPath).Return(nil)
	m.u.On("CopyFile", archivePath, pluginPath+".vsix").Return(nil)
	m.cb.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.cb.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.u.On("Download", vsixURL, archivePath).Return(nil)
	m.u.On("TempDir", "", "vscode-extension-broker").Return(workDir, nil)
	m.randMock.On("IntFromRange", 4000, 10000).Return(4242)
	m.randMock.On("String", 10).Return("randomEndpointName")
	m.randMock.On("String", 6).Return("randomContainerSuffix")
}

func setUpDownloadFailureCase(workDir string, m *mocks) {
	archivePath := filepath.Join(workDir, "pluginArchive")
	m.cb.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.cb.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.u.On("Download", vsixBrokenURL, archivePath).Return(errors.New("Failed to download plugin")).Once()
	m.u.On("TempDir", "", "vscode-extension-broker").Return(workDir, nil).Once()
	m.randMock.On("IntFromRange", 4000, 10000).Return(4242).Once()
	m.randMock.On("String", 10).Return("randomEndpointName").Once()
	m.randMock.On("String", 6).Return("randomContainerSuffix").Once()
}

func TestFetchExtensionInfo(t *testing.T) {
	cases := []struct {
		want    []byte
		ext     string
		err     string
		roundTF test.RoundTripFunc
	}{
		{
			err: "VS Code extension id 'invalidExt' parsing failed for plugin tid:tv",
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
				ioUtil:  files.New(),
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
