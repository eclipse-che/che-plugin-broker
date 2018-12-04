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

package broker

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	tests "github.com/eclipse/che-plugin-broker/brokers_test"
	cmock "github.com/eclipse/che-plugin-broker/common/mocks"
	fmock "github.com/eclipse/che-plugin-broker/files/mocks"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
)

var (
	bMock      = &cmock.Broker{}
	uMock      = &fmock.IoUtil{}
	mockBroker = &TheiaPluginBroker{
		bMock,
		uMock,
		storage.New(),
	}
)

func Test_process_remote_plugin(t *testing.T) {
	mockBroker = &TheiaPluginBroker{
		bMock,
		uMock,
		storage.New(),
	}
	workDir := tests.CreateTestWorkDir()
	defer tests.RemoveAll(workDir)
	archivePath := filepath.Join(workDir, "pluginArchive")
	unarchivedPath := filepath.Join(workDir, "plugin")
	packageJSONPath := filepath.Join(unarchivedPath, "package.json")
	meta := model.PluginMeta{
		ID:      "test-id",
		Version: "test-v",
		URL:     "http://test.url",
	}
	pluginPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s", meta.ID, meta.Version))
	packageJSON := packageJson{
		Name:      "test-name",
		Publisher: "test-publisher",
		Engines: engines{
			CheRuntimeContainer: "test/test:latest",
		},
	}
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	uMock.On("TempDir", "", "theia-plugin-broker").Return(workDir, nil).Once()
	uMock.On("Download", "http://test.url", archivePath).Return(nil).Once()
	uMock.On("Unzip", archivePath, unarchivedPath).Once().Return(func(archive string, dest string) error {
		tests.CreateDirByPath(dest)
		tests.CreateFileWithContent(packageJSONPath, tests.ToJSONQuiet(packageJSON))
		return nil
	}).Once()
	uMock.On("CopyResource", unarchivedPath, pluginPath).Return(nil).Once()

	err := mockBroker.processPlugin(meta)

	assert.Nil(t, err)
	pluginsPointer, err := mockBroker.storage.Plugins()
	assert.Nil(t, err)
	assert.NotNil(t, pluginsPointer)
	plugins := *pluginsPointer
	// get port since it is random and is used in names generation
	port := plugins[0].Endpoints[0].TargetPort
	assert.True(t, port >= 4000 && port <= 6000)
	// name contains random part, so path it to expected object generation
	containerName := plugins[0].Containers[0].Name
	expected := expectedPlugins(meta, port, packageJSON.Engines.CheRuntimeContainer, containerName, packageJSON.Publisher, packageJSON.Name)
	assert.Equal(t, expected, plugins)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func expectedPlugins(meta model.PluginMeta, port int, image string, cname string, publisher string, pubName string) []model.ChePlugin {
	sPort := strconv.Itoa(port)
	endpointName := "port" + sPort
	var re = regexp.MustCompile(`[^a-z_0-9]+`)
	prettyID := re.ReplaceAllString(publisher+"_"+pubName, `_`)
	expectedPlugins := []model.ChePlugin{
		{
			ID:      meta.ID,
			Version: meta.Version,
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

func Test_process_regular_plugin(t *testing.T) {
	mockBroker = &TheiaPluginBroker{
		bMock,
		uMock,
		storage.New(),
	}
	workDir := tests.CreateTestWorkDir()
	defer tests.RemoveAll(workDir)
	archivePath := filepath.Join(workDir, "pluginArchive")
	unarchivedPath := filepath.Join(workDir, "plugin")
	packageJSONPath := filepath.Join(unarchivedPath, "package.json")
	meta := model.PluginMeta{
		ID:      "test-id",
		Version: "test-v",
		URL:     "http://test.url",
	}
	pluginPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s.theia", meta.ID, meta.Version))
	packageJSON := packageJson{
		Name:      "test-name",
		Publisher: "test-publisher",
		Engines: engines{
			CheRuntimeContainer: "",
		},
	}
	expectedPlugins := []model.ChePlugin{
		{
			ID:      meta.ID,
			Version: meta.Version,
		},
	}
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	uMock.On("TempDir", "", "theia-plugin-broker").Return(workDir, nil).Once()
	uMock.On("Download", "http://test.url", archivePath).Return(nil).Once()
	uMock.On("Unzip", archivePath, unarchivedPath).Once().Return(func(archive string, dest string) error {
		tests.CreateDirByPath(dest)
		tests.CreateFileWithContent(packageJSONPath, tests.ToJSONQuiet(packageJSON))
		return nil
	}).Once()
	uMock.On("CopyFile", archivePath, pluginPath).Return(nil).Once()

	err := mockBroker.processPlugin(meta)

	assert.Nil(t, err)
	plugins, err := mockBroker.storage.Plugins()
	assert.Nil(t, err)
	assert.Equal(t, expectedPlugins, *plugins)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func Test_start(t *testing.T) {
	bMock.On("PubStarted").Once()
	bMock.On("PubDone", mock.AnythingOfType("string")).Once()
	bMock.On("PrintInfo", mock.AnythingOfType("string"))
	bMock.On("PrintPlan", mock.AnythingOfType("[]model.PluginMeta")).Once()
	bMock.On("CloseConsumers").Once()

	mockBroker.Start([]model.PluginMeta{})

	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func Test_process_plugin_error_if_archive_unpacking_fails(t *testing.T) {
	workDir := tests.CreateTestWorkDir()
	defer tests.RemoveAll(workDir)
	archivePath := filepath.Join(workDir, "pluginArchive")
	unarchivedPath := filepath.Join(workDir, "plugin")
	meta := model.PluginMeta{
		ID:      "test-id",
		Version: "test-v",
		URL:     "http://test.url",
	}
	uMock.On("TempDir", "", "theia-plugin-broker").Return(workDir, nil).Once()
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	uMock.On("Download", "http://test.url", archivePath).Return(nil).Once()
	uMock.On("Unzip", archivePath, unarchivedPath).Once().Return(errors.New("test"))

	err := mockBroker.processPlugin(meta)

	assert.Equal(t, errors.New("test"), err)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func Test_process_plugin_error_if_archive_downloading_fails(t *testing.T) {
	workDir := tests.CreateTestWorkDir()
	defer tests.RemoveAll(workDir)
	archivePath := filepath.Join(workDir, "pluginArchive")
	meta := model.PluginMeta{
		ID:      "test-id",
		Version: "test-v",
		URL:     "http://test.url",
	}
	uMock.On("TempDir", "", "theia-plugin-broker").Return(workDir, nil).Once()
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	uMock.On("Download", "http://test.url", archivePath).Return(errors.New("test")).Once()

	err := mockBroker.processPlugin(meta)

	assert.Equal(t, errors.New("test"), err)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func Test_process_plugin_error_if_package_JSON_missing(t *testing.T) {
	workDir := tests.CreateTestWorkDir()
	defer tests.RemoveAll(workDir)
	archivePath := filepath.Join(workDir, "pluginArchive")
	unarchivedPath := filepath.Join(workDir, "plugin")
	meta := model.PluginMeta{
		ID:      "test-id",
		Version: "test-v",
		URL:     "http://test.url",
	}
	uMock.On("TempDir", "", "theia-plugin-broker").Return(workDir, nil).Once()
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	uMock.On("Download", "http://test.url", archivePath).Return(nil).Once()
	uMock.On("Unzip", archivePath, unarchivedPath).Once().Return(func(archive string, dest string) error {
		tests.CreateDirByPath(dest)
		return nil
	}).Once()

	err := mockBroker.processPlugin(meta)

	assert.NotNil(t, err)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func Test_process_plugin_error_if_package_JSON_parsing_fails(t *testing.T) {
	workDir := tests.CreateTestWorkDir()
	defer tests.RemoveAll(workDir)
	archivePath := filepath.Join(workDir, "pluginArchive")
	unarchivedPath := filepath.Join(workDir, "plugin")
	packageJSONPath := filepath.Join(unarchivedPath, "package.json")
	meta := model.PluginMeta{
		ID:      "test-id",
		Version: "test-v",
		URL:     "http://test.url",
	}
	uMock.On("TempDir", "", "theia-plugin-broker").Return(workDir, nil).Once()
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	uMock.On("Download", "http://test.url", archivePath).Return(nil).Once()
	uMock.On("Unzip", archivePath, unarchivedPath).Once().Return(func(archive string, dest string) error {
		tests.CreateDirByPath(dest)
		tests.CreateFileWithContent(packageJSONPath, "illegal content of package.json")
		return nil
	}).Once()

	err := mockBroker.processPlugin(meta)

	assert.NotNil(t, err)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func Test_process_plugin_error_if_archive_copying_fails(t *testing.T) {
	workDir := tests.CreateTestWorkDir()
	defer tests.RemoveAll(workDir)
	archivePath := filepath.Join(workDir, "pluginArchive")
	unarchivedPath := filepath.Join(workDir, "plugin")
	packageJSONPath := filepath.Join(unarchivedPath, "package.json")
	meta := model.PluginMeta{
		ID:      "test-id",
		Version: "test-v",
		URL:     "http://test.url",
	}
	pluginPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s.theia", meta.ID, meta.Version))
	packageJSON := packageJson{
		Name:      "test-name",
		Publisher: "test-publisher",
		Engines: engines{
			CheRuntimeContainer: "",
		},
	}
	uMock.On("TempDir", "", "theia-plugin-broker").Return(workDir, nil).Once()
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	uMock.On("Download", "http://test.url", archivePath).Return(nil).Once()
	uMock.On("Unzip", archivePath, unarchivedPath).Once().Return(func(archive string, dest string) error {
		tests.CreateDirByPath(dest)
		tests.CreateFileWithContent(packageJSONPath, tests.ToJSONQuiet(packageJSON))
		return nil
	}).Once()
	uMock.On("CopyFile", archivePath, pluginPath).Return(errors.New("test error: copying archive")).Once()

	err := mockBroker.processPlugin(meta)

	assert.Equal(t, errors.New("test error: copying archive"), err)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func Test_process_plugin_error_if_archive_folder_copying_fails(t *testing.T) {
	workDir := tests.CreateTestWorkDir()
	defer tests.RemoveAll(workDir)
	archivePath := filepath.Join(workDir, "pluginArchive")
	unarchivedPath := filepath.Join(workDir, "plugin")
	packageJSONPath := filepath.Join(unarchivedPath, "package.json")
	meta := model.PluginMeta{
		ID:      "test-id",
		Version: "test-v",
		URL:     "http://test.url",
	}
	pluginPath := filepath.Join("/plugins", fmt.Sprintf("%s.%s", meta.ID, meta.Version))
	packageJSON := packageJson{
		Name:      "test-name",
		Publisher: "test-publisher",
		Engines: engines{
			CheRuntimeContainer: "test/test:latest",
		},
	}
	uMock.On("TempDir", "", "theia-plugin-broker").Return(workDir, nil).Once()
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	uMock.On("Download", "http://test.url", archivePath).Return(nil).Once()
	uMock.On("Unzip", archivePath, unarchivedPath).Once().Return(func(archive string, dest string) error {
		tests.CreateDirByPath(dest)
		tests.CreateFileWithContent(packageJSONPath, tests.ToJSONQuiet(packageJSON))
		return nil
	}).Once()
	uMock.On("CopyResource", unarchivedPath, pluginPath).Return(errors.New("test error: copying archive folder")).Once()

	err := mockBroker.processPlugin(meta)

	assert.Equal(t, errors.New("test error: copying archive folder"), err)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}
