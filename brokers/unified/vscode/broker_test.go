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
	"strings"
	"testing"

	tests "github.com/eclipse/che-plugin-broker/brokers/test"
	"github.com/eclipse/che-plugin-broker/utils"

	"github.com/eclipse/che-plugin-broker/brokers/test"
	"github.com/eclipse/che-plugin-broker/common"
	cmock "github.com/eclipse/che-plugin-broker/common/mocks"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
	fmock "github.com/eclipse/che-plugin-broker/utils/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	vsixURL          = "http://test.url"
	vsixBrokenURL    = "http://broken.test.url"
	pluginID         = "tid"
	pluginVersion    = "tv"
	image            = "test/test:tag"
	pluginPublisher  = "test publisher"
	pluginName       = "test name"
	vscodePluginType = "VS Code extension"
	theiaPluginType  = "Theia plugin"
)

type mocks struct {
	cb       *cmock.Broker
	u        *fmock.IoUtil
	b        *brokerImpl
	randMock *cmock.Random
}

func initMocks(useLocalhost bool) *mocks {
	cb := &cmock.Broker{}
	u := &fmock.IoUtil{}
	randMock := &cmock.Random{}
	return &mocks{
		cb:       cb,
		u:        u,
		randMock: randMock,
		b: &brokerImpl{
			Broker:           cb,
			ioUtil:           u,
			Storage:          storage.New(),
			client:           test.NewTestHTTPClient(okMarketplaceResponse),
			rand:             randMock,
			localhostSidecar: useLocalhost,
		},
	}
}

func TestStart(t *testing.T) {
	m := initMocks(false)

	m.cb.On("PubStarted").Once()
	m.cb.On("PrintDebug", mock.AnythingOfType("string"))
	m.cb.On("PubDone", "null").Once()
	m.cb.On("PrintInfo", mock.AnythingOfType("string"))
	m.cb.On("PrintPlan", mock.AnythingOfType("[]model.PluginMeta")).Once()
	m.cb.On("CloseConsumers").Once()

	m.b.Start([]model.PluginMeta{})

	m.cb.AssertExpectations(t)
}

func TestProcessBrokenPluginUrl(t *testing.T) {
	m := initMocks(false)

	meta := model.PluginMeta{
		ID:      pluginID,
		Version: pluginVersion,
		Spec: model.PluginMetaSpec{
			Containers: []model.Container{
				{
					Image: image,
				},
			},
			Extensions: []string{vsixBrokenURL},
		},
	}

	workDir := tests.CreateTestWorkDir()

	setUpDownloadFailureCase(workDir, m)
	m.u.On("ResolveDestPathFromURL", vsixBrokenURL, workDir).Return("/tmp/test005528325")
	defer tests.RemoveAll(workDir)
	err := m.b.ProcessPlugin(meta)

	assert.NotNil(t, err)
	assert.EqualError(t, err, "Failed to download plugin")

	plugins, err := m.b.Storage.Plugins()
	assert.Nil(t, err)
	assert.Equal(t, 0, len(plugins))

	m.cb.AssertExpectations(t)
	m.u.AssertExpectations(t)

	m.u.AssertNotCalled(t, "Unzip", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.u.AssertNotCalled(t, "CopyResource", mock.AnythingOfType("string"), mock.AnythingOfType("string"))
}

func TestBroker_processPlugin(t *testing.T) {
	cases := []struct {
		name         string
		meta         model.PluginMeta
		err          string
		want         []model.ChePlugin
		useLocalhost bool
	}{
		{
			name: "Return error when extensions field is empty and there is no containers",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
			},
			err: fmt.Sprintf(errorNoExtFieldsTemplate, "tid"),
		},
		{
			name: "Return error when extensions field is empty and there is a container",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Containers: []model.Container{
						{
							Image: image,
						},
					},
				},
			},
			err: fmt.Sprintf(errorNoExtFieldsTemplate, "tid"),
		},
		{
			name: "Successful brokering of local plugin with extensions field",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Extensions: []string{
						"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
					},
				},
			},
			want: expectedNoPlugin(),
		},
		{
			name: "Successful brokering of local plugin with extensions field and empty containers field",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Extensions: []string{
						"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
					},
					Containers: []model.Container{},
				},
			},
			want: expectedNoPlugin(),
		},
		{
			name: "Successful brokering of local plugin when extension points to .theia archive",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Extensions: []string{
						"https://red-hot-chilli.peppers/plugin.theia",
					},
					Containers: []model.Container{},
				},
			},
			want: expectedNoPlugin(),
		},
		{
			name: "Successful brokering of remote plugin with initContainers and persisted volume",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Extensions: []string{
						"https://red-hot-chilli.peppers/plugin.theia",
					},
					Containers: []model.Container{
						{
							Image: image,
						},
					},
					InitContainers: []model.Container{
						{
							Image: image,
							Volumes: []model.Volume{
								{
									Name:          "Volume-for-init-container",
									MountPath:     "/test-volume",
									PersistVolume: true,
								},
							},
							Env: []model.EnvVar{
								{
									Name:  "ExecBin",
									Value: "/test-volume/someExecBin",
								},
							},
						},
					},
				},
			},
			useLocalhost: false,
			want:         expectedPluginsWithSingleRemotePluginWithInitContainer(true),
		},
		{
			name: "Successful brokering of remote plugin with initContainers and ephemeral volume",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Extensions: []string{
						"https://red-hot-chilli.peppers/plugin.theia",
					},
					Containers: []model.Container{
						{
							Image: image,
						},
					},
					InitContainers: []model.Container{
						{
							Image: image,
							Volumes: []model.Volume{
								{
									Name:          "Volume-for-init-container",
									MountPath:     "/test-volume",
									PersistVolume: false,
								},
							},
							Env: []model.EnvVar{
								{
									Name:  "ExecBin",
									Value: "/test-volume/someExecBin",
								},
							},
						},
					},
				},
			},
			useLocalhost: false,
			want:         expectedPluginsWithSingleRemotePluginWithInitContainer(false),
		},
		{
			name: "Successful brokering of remote plugin when extension points to .theia archive, using a generated host name",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Extensions: []string{
						"https://red-hot-chilli.peppers/plugin.theia",
					},
					Containers: []model.Container{
						{
							Image: image,
						},
					},
				},
			},
			useLocalhost: false,
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				false),
		},
		{
			name: "Successful brokering of remote plugin when extension points to .theia archive, using localhost as the host name",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Extensions: []string{
						"https://red-hot-chilli.peppers/plugin.theia",
					},
					Containers: []model.Container{
						{
							Image: image,
						},
					},
				},
			},
			useLocalhost: true,
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				true),
		},
		{
			name: "Successful brokering of local plugin with extensions field with several extensions",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Extensions: []string{
						"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
						"vscode:extension/redhat-com.vscode-jdt-ls",
						"vscode:extension/redhat-com.vscode-maven",
					},
				},
			},
			want: expectedNoPlugin(),
		},
		{
			name: "Successful brokering of local plugin with extensions field with several archives URLs",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Extensions: []string{
						vsixURL,
						"https://test.url.vsix",
						"http://red-hot-chilli.peppers/cool-extension.vsix",
					},
				},
			},
			want: expectedNoPlugin(),
		},
		{
			name: "Successful brokering of local plugin with extensions field with mixed extensions and archives URLs",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Extensions: []string{
						"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
						vsixURL,
						"vscode:extension/redhat-com.vscode-maven",
					},
				},
			},
			want: expectedNoPlugin(),
		},
		{
			name: "Successful brokering of remote plugin with extensions field with several extensions, using a generated host name",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Extensions: []string{
						"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
					},
					Containers: []model.Container{
						{
							Image: image,
						},
					},
				},
			},
			useLocalhost: false,
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				false),
		},
		{
			name: "Successful brokering of remote plugin with extensions field with several extensions, using localhost as the host name",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Extensions: []string{
						"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
					},
					Containers: []model.Container{
						{
							Image: image,
						},
					},
				},
			},
			useLocalhost: true,
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				true),
		},
		{
			name: "Successful brokering of remote plugin with extensions field with several extensions, using a generated the host name",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Extensions: []string{
						"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
						"vscode:extension/redhat-com.vscode-jdt-ls",
						"vscode:extension/redhat-com.vscode-maven",
					},
					Containers: []model.Container{
						{
							Image: image,
						},
					},
				},
			},
			useLocalhost: false,
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				false),
		},
		{
			name: "Successful brokering of remote plugin with extensions field with several extensions, using localhost as the host name",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Extensions: []string{
						"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
						"vscode:extension/redhat-com.vscode-jdt-ls",
						"vscode:extension/redhat-com.vscode-maven",
					},
					Containers: []model.Container{
						{
							Image: image,
						},
					},
				},
			},
			useLocalhost: true,
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				true),
		},
		{
			name: "Successful brokering of remote plugin with extensions field with mixed extensions and archives URLs, using a generated host name",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Containers: []model.Container{
						{
							Image: image,
						},
					},
					Extensions: []string{
						"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
						vsixURL,
						"vscode:extension/redhat-com.vscode-maven",
					},
				},
			},
			useLocalhost: false,
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				false),
		},
		{
			name: "Successful brokering of remote plugin with extensions field with mixed extensions and archives URLs, using localhost as the host name",
			meta: model.PluginMeta{
				Type:      vscodePluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Containers: []model.Container{
						{
							Image: image,
						},
					},
					Extensions: []string{
						"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
						vsixURL,
						"vscode:extension/redhat-com.vscode-maven",
					},
				},
			},
			useLocalhost: true,
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				true),
		},
		{
			name: "Successful brokering of remote plugin with extensions field with mixed extensions and archives URLs when plugin type is Theia, using a generated host name",
			meta: model.PluginMeta{
				Type:      theiaPluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Containers: []model.Container{
						{
							Image: image,
						},
					},
					Extensions: []string{
						"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
						vsixURL,
						"https://red-hot-chilli.peppers/plugin.theia",
					},
				},
			},
			useLocalhost: false,
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				false),
		},
		{
			name: "Successful brokering of remote plugin with extensions field with mixed extensions and archives URLs when plugin type is Theia, using localhost as the host name",
			meta: model.PluginMeta{
				Type:      theiaPluginType,
				ID:        pluginID,
				Version:   pluginVersion,
				Publisher: pluginPublisher,
				Name:      pluginName,
				Spec: model.PluginMetaSpec{
					Containers: []model.Container{
						{
							Image: image,
						},
					},
					Extensions: []string{
						"vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
						vsixURL,
						"https://red-hot-chilli.peppers/plugin.theia",
					},
				},
			},
			useLocalhost: true,
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				true),
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			m := initMocks(tt.useLocalhost)
			workDir := tests.CreateTestWorkDir()
			defer tests.RemoveAll(workDir)
			setUpSuccessfulCase(workDir, tt.meta, m)

			if tt.want == nil && tt.err == "" {
				t.Fatal("Neither want nor error are defined")
			}
			m.u.On("ResolveDestPathFromURL", mock.AnythingOfType("string"), workDir).Return("/tmp/test005528325")
			m.u.On("MkDir", mock.AnythingOfType("string")).Return(nil)
			err := m.b.ProcessPlugin(tt.meta)
			if err != nil {
				if tt.err != "" {
					assert.EqualError(t, err, tt.err)
				} else {
					t.Errorf("ProcessPlugin() error = %v, wanted error %v", err, tt.err)
					return
				}
			} else {
				plugins, err := m.b.Storage.Plugins()
				assert.Nil(t, err)
				assert.ElementsMatch(t, tt.want, plugins)
			}
		})
	}
}

func expectedPluginsWithSingleRemotePluginWithInitContainer(volumeIsPersisted bool) []model.ChePlugin {
	expectedPlugin := model.ChePlugin{
		ID:        pluginID,
		Version:   pluginVersion,
		Publisher: pluginPublisher,
		Name:      pluginName,
		Containers: []model.Container{
			{
				Image: image,
				Volumes: []model.Volume{
					{
						Name:      "plugins",
						MountPath: "/plugins",
						PersistVolume: true,
					},
				},
				MountSources: true,
			},
		},
		InitContainers: []model.Container{
			{
				Image: image,
				Volumes: []model.Volume{
					{
						Name:          "Volume-for-init-container",
						MountPath:     "/test-volume",
						PersistVolume: volumeIsPersisted,
					},
				},
				Env: []model.EnvVar{
					{
						Name:  "ExecBin",
						Value: "/test-volume/someExecBin",
					},
				},
			},
		},
	}

	expectedPlugin.Containers[0].Ports = []model.ExposedPort{
		{
			ExposedPort: 4242,
		},
	}
	expectedPlugin.Containers[0].Env = []model.EnvVar{
		{
			Name:  "THEIA_PLUGIN_ENDPOINT_PORT",
			Value: "4242",
		},
	}
	expectedPlugin.Endpoints = []model.Endpoint{
		model.Endpoint{
			Name:       "randomString1234567890",
			Public:     false,
			TargetPort: 4242,
		},
	}
	expectedPlugin.WorkspaceEnv = append(expectedPlugin.WorkspaceEnv, model.EnvVar{
		Name:  "THEIA_PLUGIN_REMOTE_ENDPOINT_" + strings.ReplaceAll(pluginPublisher+"_"+pluginName+"_"+pluginVersion, " ", "_"),
		Value: "ws://randomString1234567890:4242",
	})

	expectedPlugin.Containers[0].Env = append(expectedPlugin.Containers[0].Env, model.EnvVar{
		Name:  "THEIA_PLUGINS",
		Value: "local-dir:///plugins/sidecars/" + getPluginUniqueName(expectedPlugin),
	})
	return []model.ChePlugin{
		expectedPlugin,
	}
}

func expectedPluginsWithSingleRemotePluginWithSeveralExtensions(usedLocalhost bool) []model.ChePlugin {
	expectedPlugin := model.ChePlugin{
		ID:        pluginID,
		Version:   pluginVersion,
		Publisher: pluginPublisher,
		Name:      pluginName,
		Containers: []model.Container{
			{
				Image: image,
				Volumes: []model.Volume{
					{
						Name:      "plugins",
						MountPath: "/plugins",
						PersistVolume: true,
					},
				},
				MountSources: true,
			},
		},
	}
	if !usedLocalhost {
		expectedPlugin.Containers[0].Ports = []model.ExposedPort{
			{
				ExposedPort: 4242,
			},
		}
		expectedPlugin.Containers[0].Env = []model.EnvVar{
			{
				Name:  "THEIA_PLUGIN_ENDPOINT_PORT",
				Value: "4242",
			},
		}
		expectedPlugin.Endpoints = []model.Endpoint{
			model.Endpoint{
				Name:       "randomString1234567890",
				Public:     false,
				TargetPort: 4242,
			},
		}
		expectedPlugin.WorkspaceEnv = append(expectedPlugin.WorkspaceEnv, model.EnvVar{
			Name:  "THEIA_PLUGIN_REMOTE_ENDPOINT_" + strings.ReplaceAll(pluginPublisher+"_"+pluginName+"_"+pluginVersion, " ", "_"),
			Value: "ws://randomString1234567890:4242",
		})
	}
	expectedPlugin.Containers[0].Env = append(expectedPlugin.Containers[0].Env, model.EnvVar{
		Name:  "THEIA_PLUGINS",
		Value: "local-dir:///plugins/sidecars/" + getPluginUniqueName(expectedPlugin),
	})
	return []model.ChePlugin{
		expectedPlugin,
	}
}

func expectedNoPlugin() []model.ChePlugin {
	return []model.ChePlugin{}
}

func setUpSuccessfulCase(workDir string, meta model.PluginMeta, m *mocks) {
	m.u.On("CopyResource", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
	pluginPath := "/plugins"
	if len(meta.Spec.Containers) > 0 {
		pluginPath = filepath.Join(pluginPath, "sidecars",
			re.ReplaceAllString(meta.Publisher+"_"+meta.Name+"_"+meta.Version, `_`))
	}
	pluginPath = filepath.Join(
		pluginPath,
		fmt.Sprintf("%s.%s.%s.randomString1234567890.test005528325", meta.Publisher, meta.Name, meta.Version))
	m.u.On("CopyFile", mock.AnythingOfType("string"), pluginPath).Return(nil)
	m.cb.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.cb.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.cb.On("PrintInfo", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.u.On("Download", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("bool")).Return("test005528325", nil)
	m.u.On("TempDir", "", "vscode-extension-broker").Return(workDir, nil)
	m.randMock.On("IntFromRange", 4000, 10000).Return(4242)
	m.randMock.On("String", 10).Return("randomString1234567890")
	m.randMock.On("String", 6).Return("randomString123456")
}

func setUpDownloadFailureCase(workDir string, m *mocks) {
	m.cb.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.cb.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.cb.On("PrintInfo", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.u.On("Download", vsixBrokenURL, mock.AnythingOfType("string"), mock.AnythingOfType("bool")).Return("", errors.New("Failed to download plugin")).Once()
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
			err: "Parsing of VS Code extension ID 'invalidExt' failed for plugin 'tid'. Extension should start from 'vscode:extension/'",
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
			err: "VS Code extension downloading failed tid. Status: 400. Body: ",
			roundTF: func(req *http.Request) *http.Response {
				return &http.Response{
					StatusCode: 400,
					Body:       ioutil.NopCloser(bytes.NewBufferString("")),
				}
			},
		},
		{
			ext: "vscode:extension/ms-kubernetes-tools.vscode-kubernetes-tools",
			err: "VS Code extension downloading failed tid. Status: 400. Body: " + "test error",
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
			var b = &brokerImpl{
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
			err:      "Failed to parse VS Code extension marketplace response for plugin tid",
			response: []byte("{"),
		},
		{
			err:      "Failed to parse VS Code extension marketplace response for plugin tid",
			response: []byte("{}"),
		},
		{
			err:      "Failed to parse VS Code extension marketplace response for plugin tid",
			response: []byte(`{"results":[]}`),
		},
		{
			err:      "Failed to parse VS Code extension marketplace response for plugin tid",
			response: []byte(`{"results":null}`),
		},
		{
			err: "Failed to parse VS Code extension marketplace response for plugin tid",
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
			err: "Failed to parse VS Code extension marketplace response for plugin tid",
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
			err: "Failed to parse VS Code extension marketplace response for plugin tid",
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
			err: "Failed to parse VS Code extension marketplace response for plugin tid",
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
			err: "Failed to parse VS Code extension marketplace response for plugin tid",
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
			err: "Failed to parse VS Code extension marketplace response for plugin tid",
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
			err: "VS Code extension archive information is not found in marketplace response for plugin tid",
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
