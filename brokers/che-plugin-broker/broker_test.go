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
	"log"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v2"

	tests "github.com/eclipse/che-plugin-broker/brokers/test"
	cmock "github.com/eclipse/che-plugin-broker/common/mocks"
	fmock "github.com/eclipse/che-plugin-broker/files/mocks"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
)

var (
	broker     = NewBroker()
	bMock      = &cmock.Broker{}
	uMock      = &fmock.IoUtil{}
	mockBroker = &ChePluginBroker{
		bMock,
		uMock,
		storage.New(),
	}
)

func TestProcessPluginErrorIfArchiveDownloadingFails(t *testing.T) {
	workDir := tests.CreateTestWorkDir()
	defer tests.RemoveAll(workDir)
	archivePath := filepath.Join(workDir, "pluginArchive.tar.gz")
	meta := model.PluginMeta{
		ID:      "test-id",
		Version: "test-v",
		URL:     "http://test.url",
	}
	uMock.On("TempDir", "", "che-plugin-broker").Return(workDir, nil).Once()
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	uMock.On("Download", "http://test.url", archivePath).Return(errors.New("test")).Once()

	err := mockBroker.processPlugin(meta)

	assert.Equal(t, errors.New("test"), err)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func TestProcessPluginErrorIfYamlDownloadingFails(t *testing.T) {
	workDir := tests.CreateTestWorkDir()
	pluginYaml := filepath.Join(workDir, pluginFileName)
	defer tests.RemoveAll(workDir)
	meta := model.PluginMeta{
		ID:      "test-id",
		Version: "test-v",
		URL:     "http://test.url/che-plugin.yaml",
	}
	uMock.On("TempDir", "", "che-plugin-broker").Return(workDir, nil).Once()
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	uMock.On("Download", "http://test.url/che-plugin.yaml", pluginYaml).Return(errors.New("test")).Once()

	err := mockBroker.processPlugin(meta)

	assert.Equal(t, errors.New("test"), err)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func TestProcessPluginErrorIfArchiveUnpackingFails(t *testing.T) {
	workDir := tests.CreateTestWorkDir()
	defer tests.RemoveAll(workDir)
	archivePath := filepath.Join(workDir, "pluginArchive.tar.gz")
	unarchivedPath := filepath.Join(workDir, "plugin")
	meta := model.PluginMeta{
		ID:      "test-id",
		Version: "test-v",
		URL:     "http://test.url",
	}
	uMock.On("TempDir", "", "che-plugin-broker").Return(workDir, nil).Once()
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	uMock.On("Download", "http://test.url", archivePath).Return(nil).Once()
	uMock.On("Untar", archivePath, unarchivedPath).Once().Return(errors.New("test"))

	err := mockBroker.processPlugin(meta)

	assert.Equal(t, errors.New("test"), err)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func TestProcessPluginErrorIfPluginYamlParsingFails(t *testing.T) {
	workDir := tests.CreateTestWorkDir()
	defer tests.RemoveAll(workDir)
	archivePath := filepath.Join(workDir, "pluginArchive.tar.gz")
	unarchivedPath := filepath.Join(workDir, "plugin")
	toolingConfPath := filepath.Join(unarchivedPath, pluginFileName)
	meta := model.PluginMeta{
		ID:          "test-id",
		Version:     "test-v",
		Description: "test description",
		Icon:        "http://test.icon",
		Name:        "test-name",
		Title:       "Test title",
		Type:        "test-type",
		URL:         "http://test.url",
	}
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	uMock.On("TempDir", "", "che-plugin-broker").Return(workDir, nil).Once()
	uMock.On("Download", "http://test.url", archivePath).Return(nil).Once()
	uMock.On("Untar", archivePath, unarchivedPath).Once().Return(func(archive string, dest string) error {
		tests.CreateDirByPath(dest)
		tests.CreateFileWithContent(toolingConfPath, "illegal yaml content")
		return nil
	})

	err := mockBroker.processPlugin(meta)

	assert.NotNil(t, err)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func TestProcessPlugin(t *testing.T) {
	workDir := tests.CreateTestWorkDir()
	mockBroker = &ChePluginBroker{
		bMock,
		uMock,
		storage.New(),
	}
	defer tests.RemoveAll(workDir)
	archivePath := filepath.Join(workDir, "pluginArchive.tar.gz")
	unarchivedPath := filepath.Join(workDir, "plugin")
	toolingConfPath := filepath.Join(unarchivedPath, pluginFileName)
	meta := model.PluginMeta{
		ID:          "test-id",
		Version:     "test-v",
		Description: "test description",
		Icon:        "http://test.icon",
		Name:        "test-name",
		Title:       "Test title",
		Type:        "test-type",
		URL:         "http://test.url",
	}
	toolingConf := model.ToolingConf{
		WorkspaceEnv: []model.EnvVar{
			{
				Name:  "envVar1",
				Value: "value1",
			},
			{
				Name:  "envVar2",
				Value: "value2",
			},
		},
		Editors: []model.Editor{},
		Containers: []model.Container{
			{
				Name:        "cname",
				Image:       "test/test:latest",
				MemoryLimit: "150Mi",
				Env: []model.EnvVar{
					{
						Name:  "envVar3",
						Value: "value3",
					},
				},
				EditorCommands: []model.EditorCommand{
					{
						Name:       "cmd1",
						WorkingDir: "/home/test",
						Command: []string{
							"ping",
							"google.com",
						},
					},
				},
				Volumes: []model.Volume{
					{
						Name:      "plugins",
						MountPath: "/plugins",
					},
				},
				Ports: []model.ExposedPort{
					{
						ExposedPort: 8080,
					},
					{
						ExposedPort: 1000,
					},
				},
			},
		},
		Endpoints: []model.Endpoint{},
	}
	expectedPlugins := []model.ChePlugin{
		{
			ID:           meta.ID,
			Version:      meta.Version,
			Endpoints:    toolingConf.Endpoints,
			Containers:   toolingConf.Containers,
			Editors:      toolingConf.Editors,
			WorkspaceEnv: toolingConf.WorkspaceEnv,
		},
	}
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	uMock.On("TempDir", "", "che-plugin-broker").Return(workDir, nil).Once()
	uMock.On("Download", "http://test.url", archivePath).Return(nil).Once()
	uMock.On("Untar", archivePath, unarchivedPath).Once().Return(func(archive string, dest string) error {
		tests.CreateDirByPath(dest)
		tests.CreateFileWithContent(toolingConfPath, tests.ToYamlQuiet(toolingConf))
		return nil
	})

	err := mockBroker.processPlugin(meta)

	assert.Nil(t, err)
	plugins, err := mockBroker.storage.Plugins()
	assert.Nil(t, err)
	assert.Equal(t, expectedPlugins, *plugins)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func TestProcessPluginWithYaml(t *testing.T) {
	workDir := tests.CreateTestWorkDir()
	pluginYaml := filepath.Join(workDir, pluginFileName)
	defer tests.RemoveAll(workDir)
	mockBroker = &ChePluginBroker{
		bMock,
		uMock,
		storage.New(),
	}
	meta := model.PluginMeta{
		ID:          "test-id",
		Version:     "test-v",
		Description: "test description",
		Icon:        "http://test.icon",
		Name:        "test-name",
		Title:       "Test title",
		Type:        "test-type",
		URL:         "http://test.url/che-plugin.yaml",
	}
	toolingConf := model.ToolingConf{
		WorkspaceEnv: []model.EnvVar{
			{
				Name:  "envVar1",
				Value: "value1",
			},
			{
				Name:  "envVar2",
				Value: "value2",
			},
		},
		Editors: []model.Editor{},
		Containers: []model.Container{
			{
				Name:        "cname",
				Image:       "test/test:latest",
				MemoryLimit: "150Mi",
				Env: []model.EnvVar{
					{
						Name:  "envVar3",
						Value: "value3",
					},
				},
				EditorCommands: []model.EditorCommand{
					{
						Name:       "cmd1",
						WorkingDir: "/home/test",
						Command: []string{
							"ping",
							"google.com",
						},
					},
				},
				Volumes: []model.Volume{
					{
						Name:      "plugins",
						MountPath: "/plugins",
					},
				},
				Ports: []model.ExposedPort{
					{
						ExposedPort: 8080,
					},
					{
						ExposedPort: 1000,
					},
				},
			},
		},
		Endpoints: []model.Endpoint{},
	}
	expectedPlugins := []model.ChePlugin{
		{
			ID:           meta.ID,
			Version:      meta.Version,
			Endpoints:    toolingConf.Endpoints,
			Containers:   toolingConf.Containers,
			Editors:      toolingConf.Editors,
			WorkspaceEnv: toolingConf.WorkspaceEnv,
		},
	}
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	uMock.On("TempDir", "", "che-plugin-broker").Return(workDir, nil).Once()
	uMock.On("Download", "http://test.url/che-plugin.yaml", pluginYaml).Return(nil).Once().Return(func(URL string, destPath string) error {
		tests.CreateFileWithContent(destPath, tests.ToYamlQuiet(toolingConf))
		return nil
	})

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

func TestCopyDependenciesDownloadsFileIfURLIsPresent(t *testing.T) {
	dep := &model.CheDependency{
		ID:      "test-id",
		Version: "test-v",
		URL:     "http://domain.com/test.theia",
	}
	workDir := createDepFile(dep)
	defer tests.RemoveAll(workDir)
	uMock.On("ResolveDestPathFromURL", "http://domain.com/test.theia", "/plugins").Return("/plugins").Once()
	uMock.On("Download", "http://domain.com/test.theia", "/plugins").Return(nil).Once()
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))

	err := mockBroker.copyDependencies(workDir)

	assert.Nil(t, err)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func TestCopyDependenciesCopiesFilesIfLocationIsPresent(t *testing.T) {
	dep := &model.CheDependency{
		ID:       "test-id",
		Version:  "test-v",
		Location: "test.theia",
	}
	workDir := createDepFile(dep)
	defer tests.RemoveAll(workDir)
	uMock.On("ResolveDestPath", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return("/plugins").Once()
	uMock.On("CopyResource", filepath.Join(workDir, "test.theia"), "/plugins").Return(nil).Once()
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))

	err := mockBroker.copyDependencies(workDir)

	assert.Nil(t, err)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func TestCopyDependenciesProcessesSeveralDeps(t *testing.T) {
	dep1 := &model.CheDependency{
		ID:       "test-id",
		Version:  "test-v",
		Location: "test.theia",
	}
	dep2 := &model.CheDependency{
		ID:      "test-id",
		Version: "test-v",
		URL:     "http://domain.com/test.theia",
	}
	workDir := createDepFile(dep1, dep2)
	defer tests.RemoveAll(workDir)
	uMock.On("ResolveDestPathFromURL", "http://domain.com/test.theia", "/plugins").Return("/plugins").Once()
	uMock.On("Download", "http://domain.com/test.theia", "/plugins").Return(nil).Once()
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))
	uMock.On("ResolveDestPath", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return("/plugins").Once()
	uMock.On("CopyResource", filepath.Join(workDir, "test.theia"), "/plugins").Return(nil).Once()

	err := mockBroker.copyDependencies(workDir)

	assert.Nil(t, err)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func TestCopyDependenciesErrorIfNeitherURLNorLocationArePresent(t *testing.T) {
	dep := &model.CheDependency{
		ID:      "test-id",
		Version: "test-v",
	}
	workDir := createDepFile(dep)
	defer tests.RemoveAll(workDir)

	err := broker.copyDependencies(workDir)

	assert.Equal(t, fmt.Errorf(depFileNoLocationURLError, dep.ID, dep.Version), err)
}

func TestCopyDependenciesErrorIfLocationAndURLArePresent(t *testing.T) {
	dep := &model.CheDependency{
		ID:       "test-id",
		Version:  "test-v",
		Location: "file.zip",
		URL:      "http://test.com/file.zip",
	}
	workDir := createDepFile(dep)
	defer tests.RemoveAll(workDir)

	err := broker.copyDependencies(workDir)

	assert.Equal(t, fmt.Errorf(depFileBothLocationAndURLError, dep.ID, dep.Version), err)
}

func TestCopyDependenciesErrorIfParsingFails(t *testing.T) {
	workDir := tests.CreateTestWorkDir()
	defer tests.RemoveAll(workDir)
	createDepFileWithIllegalContent(workDir)

	err := broker.copyDependencies(workDir)

	assert.NotNil(t, err)
}

func TestDepFileNotExist(t *testing.T) {
	dir := "/tmp/thisFileShouldNotExist"

	got, err := broker.parseDepsFile(dir)

	assert.Nil(t, err)
	assert.Nil(t, got)
}

func TestDepFileIsFolderError(t *testing.T) {
	dir := tests.CreateTestWorkDir()
	tests.CreateDir(dir, depFileName)
	defer tests.RemoveAll(dir)

	got, err := broker.parseDepsFile(dir)

	assert.Nil(t, got)
	assert.NotNil(t, err)
}

func TestDepFileIsNotReadableError(t *testing.T) {
	dir := tests.CreateTestWorkDir()
	tests.CreateFile(dir, depFileName, 0333)
	defer tests.RemoveAll(dir)

	got, err := broker.parseDepsFile(dir)

	assert.Nil(t, got)
	assert.NotNil(t, err)
}

func TestDepFileParsingFails(t *testing.T) {
	dir := tests.CreateTestWorkDir()
	createDepFileWithIllegalContent(dir)
	defer tests.RemoveAll(dir)

	got, err := broker.parseDepsFile(dir)

	assert.Nil(t, got)
	assert.NotNil(t, err)
}

func TestGetDepFile(t *testing.T) {
	dep := &model.CheDependency{
		ID:       "test-id",
		Version:  "test-version",
		Location: "test-location",
		URL:      "test-url",
	}
	workDir := createDepFile(dep)
	defer tests.RemoveAll(workDir)
	expected := &model.CheDependencies{
		Plugins: []model.CheDependency{
			*dep,
		},
	}

	got, err := broker.parseDepsFile(workDir)

	assert.Equal(t, expected, got)
	assert.NoError(t, err)
}

func createDepFile(deps ...*model.CheDependency) string {
	workDir := tests.CreateTestWorkDir()
	result := &model.CheDependencies{}
	for _, dep := range deps {
		result.Plugins = append(result.Plugins, *dep)
	}
	createDepFileWithContent(workDir, result)
	return workDir
}

func createDepFileWithContent(workDir string, obj interface{}) {
	tests.CreateFile(workDir, depFileName, 0665)
	bytes, err := yaml.Marshal(obj)
	if err != nil {
		log.Fatal(err)
	}
	tests.WriteContentBytes(workDir, depFileName, bytes)
}

func createDepFileWithIllegalContent(workDir string) {
	path := filepath.Join(workDir, depFileName)
	tests.CreateFileByPath(path)
	tests.WriteContent(path, "illegal content")
}
