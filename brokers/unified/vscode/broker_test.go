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
	tests "github.com/eclipse/che-plugin-broker/brokers/test"
	"github.com/eclipse/che-plugin-broker/utils"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"

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
	extName          = "Test-name"
	extPublisher     = "Test-publisher"
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
			Broker:  cb,
			ioUtil:  u,
			Storage: storage.New(),
			client:  test.NewTestHTTPClient(okMarketplaceResponse),
			rand:    randMock,
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
		unzipFunc    UnzipFunc
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
			unzipFunc: createUnzipTheiaArchiveFuncStub(generatePackageJSON("peppers.com", "cool-extension")),
			want:      expectedNoPlugin(),
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
			unzipFunc: createUnzipTheiaArchiveFuncStub(generatePackageJSON("peppers.com", "cool-extension")),
			useLocalhost: false,
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				false,
				generateTheiaEnvVar("peppers_com_cool_extension")),
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
			unzipFunc: createUnzipTheiaArchiveFuncStub(generatePackageJSON("peppers.com", "cool-extension")),
			useLocalhost: true,
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				true,
				generateTheiaEnvVar("peppers_com_cool_extension")),
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
			unzipFunc: createUnzipFuncStub(generatePackageJSON("ms-kubernetes-tools", "vscode-kubernetes-tools")),
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				false,
				generateTheiaEnvVar("ms_kubernetes_tools_vscode_kubernetes_tools")),
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
			unzipFunc: createUnzipFuncStub(generatePackageJSON("ms-kubernetes-tools", "vscode-kubernetes-tools")),
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				true,
				generateTheiaEnvVar("ms_kubernetes_tools_vscode_kubernetes_tools")),
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
			unzipFunc: createUnzipFuncStub(
				generatePackageJSON("ms-kubernetes-tools", "vscode-kubernetes-tools"),
				generatePackageJSON("redhat-com", "vscode-jdt-ls"),
				generatePackageJSON("redhat-com", "vscode-maven")),
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				false,
				generateTheiaEnvVar("ms_kubernetes_tools_vscode_kubernetes_tools"),
				generateTheiaEnvVar("redhat_com_vscode_jdt_ls"),
				generateTheiaEnvVar("redhat_com_vscode_maven")),
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
			unzipFunc: createUnzipFuncStub(
				generatePackageJSON("ms-kubernetes-tools", "vscode-kubernetes-tools"),
				generatePackageJSON("redhat-com", "vscode-jdt-ls"),
				generatePackageJSON("redhat-com", "vscode-maven")),
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				true,
				generateTheiaEnvVar("ms_kubernetes_tools_vscode_kubernetes_tools"),
				generateTheiaEnvVar("redhat_com_vscode_jdt_ls"),
				generateTheiaEnvVar("redhat_com_vscode_maven")),
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
			unzipFunc: createUnzipFuncStub(
				generatePackageJSON("ms-kubernetes-tools", "vscode-kubernetes-tools"),
				generatePackageJSON("redhat-com", "vscode-jdt-ls"),
				generatePackageJSON("redhat-com", "vscode-maven")),
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				false,
				generateTheiaEnvVar("ms_kubernetes_tools_vscode_kubernetes_tools"),
				generateTheiaEnvVar("redhat_com_vscode_jdt_ls"),
				generateTheiaEnvVar("redhat_com_vscode_maven")),
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
			unzipFunc: createUnzipFuncStub(
				generatePackageJSON("ms-kubernetes-tools", "vscode-kubernetes-tools"),
				generatePackageJSON("redhat-com", "vscode-jdt-ls"),
				generatePackageJSON("redhat-com", "vscode-maven")),
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				true,
				generateTheiaEnvVar("ms_kubernetes_tools_vscode_kubernetes_tools"),
				generateTheiaEnvVar("redhat_com_vscode_jdt_ls"),
				generateTheiaEnvVar("redhat_com_vscode_maven")),
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
			unzipFunc: createUnzipFuncStub(
				generatePackageJSON("ms-kubernetes-tools", "vscode-kubernetes-tools"),
				generatePackageJSON("redhat-com", "vscode-jdt-ls"),
				generatePackageJSON("peppers.com", "cool-extension"), ),
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				false,
				generateTheiaEnvVar("ms_kubernetes_tools_vscode_kubernetes_tools"),
				generateTheiaEnvVar("redhat_com_vscode_jdt_ls"),
				generateTheiaEnvVar("peppers_com_cool_extension")),
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
			unzipFunc: createUnzipFuncStub(
				generatePackageJSON("ms-kubernetes-tools", "vscode-kubernetes-tools"),
				generatePackageJSON("redhat-com", "vscode-jdt-ls"),
				generatePackageJSON("peppers.com", "cool-extension"), ),
			want: expectedPluginsWithSingleRemotePluginWithSeveralExtensions(
				true,
				generateTheiaEnvVar("ms_kubernetes_tools_vscode_kubernetes_tools"),
				generateTheiaEnvVar("redhat_com_vscode_jdt_ls"),
				generateTheiaEnvVar("peppers_com_cool_extension")),
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			m := initMocks(tt.useLocalhost)
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
				plugins, err := m.b.Storage.Plugins()
				assert.Nil(t, err)
				assert.ElementsMatch(t, tt.want, plugins)
			}
		})
	}
}

func generateTheiaEnvVar(prettyID string) string {
	return "THEIA_PLUGIN_REMOTE_ENDPOINT_" + prettyID
}

func expectedPluginsWithSingleRemotePluginWithSeveralExtensions(usedLocalhost bool, pluginTheiaEndpointVars ...string) []model.ChePlugin {
	expectedPlugin := model.ChePlugin{
		ID:        pluginID,
		Version:   pluginVersion,
		Publisher: pluginPublisher,
		Name:      pluginName,
		Endpoints: []model.Endpoint{},
		Containers: []model.Container{
			{
				Image: image,
				Volumes: []model.Volume{
					{
						Name:      "plugins",
						MountPath: "/plugins",
					},
				},
				MountSources: true,
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
	if ! usedLocalhost  {
		expectedPlugin.Endpoints = append(expectedPlugin.Endpoints, model.Endpoint{
			Name:       "randomString1234567890",
			Public:     false,
			TargetPort: 4242,
		})
	}
	for _, envVarName := range pluginTheiaEndpointVars {
		hostName := "randomString1234567890"
		if (usedLocalhost) {
			hostName = "localhost"
		}
		expectedPlugin.WorkspaceEnv = append(expectedPlugin.WorkspaceEnv, model.EnvVar{
			Name:  envVarName,
			Value: "ws://" + hostName + ":4242",
		})
	}

	return []model.ChePlugin{
		expectedPlugin,
	}
}

func expectedNoPlugin() []model.ChePlugin {
	return []model.ChePlugin{}
}

func setUpSuccessfulCase(workDir string, meta model.PluginMeta, m *mocks, unzipFunc UnzipFunc) {
	_unzipFunc := defaultUnzipFunc()
	if unzipFunc != nil {
		_unzipFunc = unzipFunc
	}
	m.u.On("Unzip", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Run(_unzipFunc).Return(nil)
	m.u.On("CopyResource", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
	pluginPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s.%s.randomString1234567890", meta.Publisher, meta.Name, meta.Version))
	m.u.On("CopyFile", mock.AnythingOfType("string"), pluginPath).Return(nil)
	m.cb.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.cb.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.cb.On("PrintInfo", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	m.u.On("Download", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
	m.u.On("TempDir", "", "vscode-extension-broker").Return(workDir, nil)
	m.randMock.On("IntFromRange", 4000, 10000).Return(4242)
	m.randMock.On("String", 10).Return("randomString1234567890")
	m.randMock.On("String", 6).Return("randomString123456")
}

type UnzipFunc func(args mock.Arguments)

func defaultUnzipFunc() UnzipFunc {
	return func(args mock.Arguments) {
		dest := args[1].(string)
		packageJSON := PackageJSON{
			Name:      extName,
			Publisher: extPublisher,
		}
		packageJSONParent := filepath.Join(dest, vsixPackageJSONFolderName)
		tests.CreateDirs(packageJSONParent)
		tests.CreateFileByPath(filepath.Join(dest, vsixManifestFileName))
		packageJSONPath := filepath.Join(packageJSONParent, "package.json")
		tests.CreateFileWithContent(packageJSONPath, tests.ToJSONQuiet(packageJSON))
	}
}

// Makes UnzipFunc create unzip results on the file system.
// When called first time creates package.json with content from first argument.
// When called second time uses second argument if present. And so on.
// When calls number is bigger than arguments number uses last argument for all the calls that do not have matching argument.
func createUnzipFuncStub(pjs ...PackageJSON) UnzipFunc {
	jsons := pjs
	return func(args mock.Arguments) {
		dest := args[1].(string)
		packageJSONParent := filepath.Join(dest, vsixPackageJSONFolderName)
		tests.CreateDirs(packageJSONParent)
		tests.CreateFileByPath(filepath.Join(dest, vsixManifestFileName))
		packageJSONPath := filepath.Join(packageJSONParent, "package.json")
		json := jsons[0]
		if len(jsons) > 1 {
			jsons = jsons[1:]
		}
		tests.CreateFileWithContent(packageJSONPath, tests.ToJSONQuiet(json))
	}
}

// Makes UnzipFunc create unzip results on the file system.
// When called first time creates package.json with content from first argument.
// When called second time uses second argument if present. And so on.
// When calls number is bigger than arguments number uses last argument for all the calls that do not have matching argument.
func createUnzipTheiaArchiveFuncStub(pjs ...PackageJSON) UnzipFunc {
	jsons := pjs
	return func(args mock.Arguments) {
		packageJSONParent := args[1].(string)
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
