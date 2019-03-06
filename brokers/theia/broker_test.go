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

package theia

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	tests "github.com/eclipse/che-plugin-broker/brokers/test"
	cmock "github.com/eclipse/che-plugin-broker/common/mocks"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
	umock "github.com/eclipse/che-plugin-broker/utils/mocks"
)

var (
	bMock      = &cmock.Broker{}
	uMock      = &umock.IoUtil{}
	randMock   = &cmock.Random{}
	mockBroker = &Broker{
		Broker:  bMock,
		ioUtil:  uMock,
		storage: storage.New(),
		rand: randMock,
	}
)

func TestProcessRemotePlugin(t *testing.T) {
	mockBroker = &Broker{
		Broker:  bMock,
		ioUtil:  uMock,
		storage: storage.New(),
		rand: randMock,
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
	packageJSON := PackageJSON{
		PackageJSON: model.PackageJSON{
			Name:      "test-name",
			Publisher: "test-publisher",
		},
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
	randMock.On("Int", 6).Return(42).Once()
	randMock.On("IntFromRange", 4000, 10000).Return(4242).Once()
	randMock.On("String", 10).Return("randomEndpointName").Once()
	randMock.On("String", 6).Return("randomContainerSuffix").Once()
	expected := expectedPlugins(meta, packageJSON.Engines.CheRuntimeContainer, packageJSON.Publisher, packageJSON.Name)

	err := mockBroker.processPlugin(meta)

	assert.Nil(t, err)
	pluginsPointer, err := mockBroker.storage.Plugins()
	assert.Nil(t, err)
	assert.NotNil(t, pluginsPointer)
	plugins := *pluginsPointer
	assert.Equal(t, expected, plugins)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func expectedPlugins(meta model.PluginMeta, image string, publisher string, pubName string) []model.ChePlugin {
	var re = regexp.MustCompile(`[^a-z_0-9]+`)
	prettyID := re.ReplaceAllString(publisher+"_"+pubName, `_`)
	expectedPlugins := []model.ChePlugin{
		{
			ID:      meta.ID,
			Version: meta.Version,
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

func TestProcessRegularPlugin(t *testing.T) {
	mockBroker = &Broker{
		Broker:  bMock,
		ioUtil:  uMock,
		storage: storage.New(),
		rand: randMock,
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
	packageJSON := PackageJSON{
		PackageJSON: model.PackageJSON{
			Name:      "test-name",
			Publisher: "test-publisher",
		},
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

func TestStart(t *testing.T) {
	bMock.On("PubStarted").Once()
	bMock.On("PubDone", mock.AnythingOfType("string")).Once()
	bMock.On("PrintInfo", mock.AnythingOfType("string"))
	bMock.On("PrintPlan", mock.AnythingOfType("[]model.PluginMeta")).Once()
	bMock.On("CloseConsumers").Once()

	mockBroker.Start([]model.PluginMeta{})

	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func TestProcessPluginErrorIfArchiveUnpackingFails(t *testing.T) {
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

func TestProcessPluginErrorIfArchiveDownloadingFails(t *testing.T) {
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

func TestProcessPluginErrorIfPackageJSONMissing(t *testing.T) {
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

func TestProcessPluginErrorIfPackageJSONParsingFails(t *testing.T) {
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

func TestProcessPluginErrorIfArchiveCopyingFails(t *testing.T) {
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
	packageJSON := PackageJSON{
		PackageJSON: model.PackageJSON{
			Name:      "test-name",
			Publisher: "test-publisher",
		},
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

func TestProcessPluginErrorIfArchiveFolderCopyingFails(t *testing.T) {
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
	packageJSON := PackageJSON{
		PackageJSON: model.PackageJSON{
			Name:      "test-name",
			Publisher: "test-publisher",
		},
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
