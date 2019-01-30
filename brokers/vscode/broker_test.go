//
// Copyright (c) 2012-2018 Red Hat, Inc.
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
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/eclipse/che-plugin-broker/brokers/test"
	tests "github.com/eclipse/che-plugin-broker/brokers/test"
	"github.com/eclipse/che-plugin-broker/brokers/theia"
	"github.com/eclipse/che-plugin-broker/common"
	cmock "github.com/eclipse/che-plugin-broker/common/mocks"
	"github.com/eclipse/che-plugin-broker/files"
	fmock "github.com/eclipse/che-plugin-broker/files/mocks"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
)

const (
	extName       = "Test-name"
	extPublisher  = "Test-publisher"
	vsixURL       = "http://test.url"
	pluginID      = "tid"
	pluginVersion = "tv"
	image         = "test/test:tag"
)

type mocks struct {
	cb *cmock.Broker
	u  *fmock.IoUtil
	b  *VSCodeExtensionBroker
}

func initMocks() *mocks {
	cb := &cmock.Broker{}
	u := &fmock.IoUtil{}
	return &mocks{
		cb: cb,
		u:  u,
		b: &VSCodeExtensionBroker{
			cb,
			u,
			storage.New(),
			test.NewTestHTTPClient(okMarketplaceResponse),
		},
	}
}

func TestStart(t *testing.T) {
	mocks := initMocks()

	mocks.cb.On("PubStarted").Once()
	mocks.cb.On("PrintDebug", mock.AnythingOfType("string"))
	mocks.cb.On("PubDone", "null").Once()
	mocks.cb.On("PrintInfo", mock.AnythingOfType("string"))
	mocks.cb.On("PrintPlan", mock.AnythingOfType("[]model.PluginMeta")).Once()
	mocks.cb.On("CloseConsumers").Once()

	mocks.b.Start([]model.PluginMeta{})

	mocks.cb.AssertExpectations(t)
	mocks.u.AssertExpectations(t)
}

func TestProcessPlugin(t *testing.T) {
	m := initMocks()
	meta := model.PluginMeta{
		ID:      pluginID,
		Version: pluginVersion,
		Attributes: map[string]string{
			"extension":       "vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
			"containerImage": image,
		},
	}

	workDir := tests.CreateTestWorkDir()
	setUp(workDir, meta, m)
	defer tests.RemoveAll(workDir)

	err := m.b.processPlugin(meta)

	assert.Nil(t, err)

	validatePluginConfig(t, m)
	m.cb.AssertExpectations(t)
	m.u.AssertExpectations(t)
}

func TestProcessPluginNoExtension(t *testing.T) {
	m := initMocks()
	meta := model.PluginMeta{
		ID:      pluginID,
		Version: pluginVersion,
		Attributes: map[string]string{
			"containerImage": image,
		},
	}

	workDir := tests.CreateTestWorkDir()
	setUp(workDir, meta, m)
	defer tests.RemoveAll(workDir)

	err := m.b.processPlugin(meta)

	assert.EqualError(t, err, "VS Code extension field 'extension' is missing in description of plugin tid:tv")
}

func TestProcessPluginNoImage(t *testing.T) {
	m := initMocks()
	meta := model.PluginMeta{
		ID:      pluginID,
		Version: pluginVersion,
		Attributes: map[string]string{
			"extension": "vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
		},
	}

	workDir := tests.CreateTestWorkDir()
	setUp(workDir, meta, m)
	defer tests.RemoveAll(workDir)

	err := m.b.processPlugin(meta)

	assert.EqualError(t, err, "VS Code extension field 'containerImage' is missing in description of plugin tid:tv")
}

func TestProcessPluginNoAttributes(t *testing.T) {
	m := initMocks()
	meta := model.PluginMeta{
		ID:      pluginID,
		Version: pluginVersion,
	}

	workDir := tests.CreateTestWorkDir()
	setUp(workDir, meta, m)
	defer tests.RemoveAll(workDir)

	err := m.b.processPlugin(meta)

	assert.EqualError(t, err, "VS Code extension field 'extension' is missing in description of plugin tid:tv")
}

func validatePluginConfig(t *testing.T, m *mocks) {
	pluginsPointer, err := m.b.Storage.Plugins()
	assert.Nil(t, err)
	assert.NotNil(t, pluginsPointer)
	plugins := *pluginsPointer
	// get port since it is random and is used in names generation
	port := plugins[0].Endpoints[0].TargetPort
	assert.True(t, port >= 4000 && port <= 6000)
	// name contains random part, so use it to expected object generation
	containerName := plugins[0].Containers[0].Name
	expected := expectedPlugins(port, containerName)
	assert.Equal(t, expected, plugins)
}

func setUp(workDir string, meta model.PluginMeta, m *mocks) {
	archivePath := filepath.Join(workDir, "pluginArchive")
	unarchivedPath := filepath.Join(workDir, "plugin")
	packageJSONPath := filepath.Join(unarchivedPath, "extension", "package.json")
	pluginPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s", meta.ID, meta.Version))
	packageJSON := theia.PackageJSON{
		Name:      extName,
		Publisher: extPublisher,
	}
	m.u.On("Unzip", archivePath, unarchivedPath).Once().Return(func(archive string, dest string) error {
		tests.CreateDirs(filepath.Join(dest, "extension"))
		tests.CreateFileWithContent(packageJSONPath, tests.ToJSONQuiet(packageJSON))
		return nil
	}).Once()
	m.u.On("CopyResource", unarchivedPath, pluginPath).Return(nil).Once()
	m.cb.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.cb.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.u.On("Download", vsixURL, archivePath).Return(nil).Once()
	m.u.On("TempDir", "", "vscode-extension-broker").Return(workDir, nil).Once()
}

func TestFetchExtensionInfo(t *testing.T) {
	tests := []struct {
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
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if tt.want == nil && tt.err == "" {
				t.Fatal("Neither want nor error are defined")
			}
			var b = &VSCodeExtensionBroker{
				common.NewBroker(),
				files.New(),
				storage.New(),
				test.NewTestHTTPClient(tt.roundTF),
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
	tests := []struct {
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
	for _, tt := range tests {
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

func expectedPlugins(port int, cname string) []model.ChePlugin {
	sPort := strconv.Itoa(port)
	endpointName := "port" + sPort
	prettyID := "Test_publisher_Test_name"
	expectedPlugins := []model.ChePlugin{
		{
			ID:      pluginID,
			Version: pluginVersion,
			Endpoints: []model.Endpoint{
				{
					Name:       endpointName,
					Public:     false,
					TargetPort: port,
				},
			},
			Containers: []model.Container{
				{
					Name:  cname,
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
							ExposedPort: port,
						},
					},
					Env: []model.EnvVar{
						{
							Name:  "THEIA_PLUGIN_ENDPOINT_PORT",
							Value: sPort,
						},
					},
				},
			},
			WorkspaceEnv: []model.EnvVar{
				{
					Name:  "THEIA_PLUGIN_REMOTE_ENDPOINT_" + prettyID,
					Value: "ws://" + endpointName + ":" + sPort,
				},
			},
		},
	}
	return expectedPlugins
}
