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
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	yaml "gopkg.in/yaml.v2"

	"github.com/eclipse/che-plugin-broker/common"
	cmock "github.com/eclipse/che-plugin-broker/common/mocks"
	"github.com/eclipse/che-plugin-broker/files"
	fmock "github.com/eclipse/che-plugin-broker/files/mocks"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
)

var (
	broker = &ChePluginBroker{
		common.NewBroker(),
		files.New(),
	}
	bMock      = &cmock.Broker{}
	uMock      = &fmock.IoUtil{}
	mockBroker = &ChePluginBroker{
		bMock,
		uMock,
	}
)

func Test_process_plugin_error_if_archive_downloading_fails(t *testing.T) {
	workDir := createTestWorkDir()
	defer removeAll(workDir)
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

func Test_process_plugin_error_if_archive_unpacking_fails(t *testing.T) {
	workDir := createTestWorkDir()
	defer removeAll(workDir)
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

func Test_process_plugin_error_if_plugin_yaml_parsing_fails(t *testing.T) {
	workDir := createTestWorkDir()
	defer removeAll(workDir)
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
		createDirByPath(dest)
		createFileWithContent(toolingConfPath, "illegal yaml content")
		return nil
	})

	err := mockBroker.processPlugin(meta)

	assert.NotNil(t, err)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func Test_process_plugin(t *testing.T) {
	workDir := createTestWorkDir()
	defer removeAll(workDir)
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
		createDirByPath(dest)
		createFileWithContent(toolingConfPath, toYamlQuiet(toolingConf))
		return nil
	})

	err := mockBroker.processPlugin(meta)

	assert.Nil(t, err)
	plugins, err := storage.Plugins()
	assert.Equal(t, expectedPlugins, *plugins)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func Test_start(t *testing.T) {
	bMock.On("PubStarted").Once()
	bMock.On("PubDone", mock.AnythingOfType("string")).Once()
	bMock.On("PrintInfo", mock.AnythingOfType("string"))
	uMock.On("ClearDir", "/plugins").Return(nil).Once()
	bMock.On("PrintPlan", mock.AnythingOfType("[]model.PluginMeta")).Once()
	bMock.On("CloseConsumers").Once()

	mockBroker.Start([]model.PluginMeta{})

	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func Test_copy_dependencies_downloads_file_if_URL_is_present(t *testing.T) {
	dep := &model.CheDependency{
		ID:      "test-id",
		Version: "test-v",
		URL:     "http://domain.com/test.theia",
	}
	workDir := createDepFile(dep)
	defer removeAll(workDir)
	uMock.On("ResolveDestPathFromURL", "http://domain.com/test.theia", "/plugins").Return("/plugins").Once()
	uMock.On("Download", "http://domain.com/test.theia", "/plugins").Return(nil).Once()
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))

	err := mockBroker.copyDependencies(workDir)

	assert.Nil(t, err)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func Test_copy_dependencies_copies_files_if_location_is_present(t *testing.T) {
	dep := &model.CheDependency{
		ID:       "test-id",
		Version:  "test-v",
		Location: "test.theia",
	}
	workDir := createDepFile(dep)
	defer removeAll(workDir)
	uMock.On("ResolveDestPath", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return("/plugins").Once()
	uMock.On("CopyResource", filepath.Join(workDir, "test.theia"), "/plugins").Return(nil).Once()
	bMock.On("PrintDebug", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"))

	err := mockBroker.copyDependencies(workDir)

	assert.Nil(t, err)
	bMock.AssertExpectations(t)
	uMock.AssertExpectations(t)
}

func Test_copy_dependencies_processes_several_deps(t *testing.T) {
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
	defer removeAll(workDir)
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

func Test_copy_dependencies_error_if_neither_URL_nor_location_are_present(t *testing.T) {
	dep := &model.CheDependency{
		ID:      "test-id",
		Version: "test-v",
	}
	workDir := createDepFile(dep)
	defer removeAll(workDir)

	err := broker.copyDependencies(workDir)

	assert.Equal(t, fmt.Errorf(depFileNoLocationURLError, dep.ID, dep.Version), err)
}

func Test_copy_dependencies_error_if_location_and_URL_are_present(t *testing.T) {
	dep := &model.CheDependency{
		ID:       "test-id",
		Version:  "test-v",
		Location: "file.zip",
		URL:      "http://test.com/file.zip",
	}
	workDir := createDepFile(dep)
	defer removeAll(workDir)

	err := broker.copyDependencies(workDir)

	assert.Equal(t, fmt.Errorf(depFileBothLocationAndURLError, dep.ID, dep.Version), err)
}

func Test_copy_dependencies_error_if_parsing_fails(t *testing.T) {
	workDir := createTestWorkDir()
	defer removeAll(workDir)
	createDepFileWithIllegalContent(workDir)

	err := broker.copyDependencies(workDir)

	assert.NotNil(t, err)
}

func Test_dep_file_not_exist(t *testing.T) {
	dir := "/tmp/thisFileShouldNotExist"

	got, err := broker.parseDepsFile(dir)

	assert.Nil(t, err)
	assert.Nil(t, got)
}

func Test_dep_file_is_folder_error(t *testing.T) {
	dir := createTestWorkDir()
	createDir(dir, depFileName)
	defer removeAll(dir)

	got, err := broker.parseDepsFile(dir)

	assert.Nil(t, got)
	assert.NotNil(t, err)
}

func Test_dep_file_is_not_readable_error(t *testing.T) {
	dir := createTestWorkDir()
	createFile(dir, depFileName, 0337)
	defer removeAll(dir)

	got, err := broker.parseDepsFile(dir)

	assert.Nil(t, got)
	assert.NotNil(t, err)
}

func Test_dep_file_parsing_fails(t *testing.T) {
	dir := createTestWorkDir()
	createDepFileWithIllegalContent(dir)
	defer removeAll(dir)

	got, err := broker.parseDepsFile(dir)

	assert.Nil(t, got)
	assert.NotNil(t, err)
}

func Test_get_dep_file(t *testing.T) {
	dep := &model.CheDependency{
		ID:       "test-id",
		Version:  "test-version",
		Location: "test-location",
		URL:      "test-url",
	}
	workDir := createDepFile(dep)
	defer removeAll(workDir)
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
	workDir := createTestWorkDir()
	result := &model.CheDependencies{}
	for _, dep := range deps {
		result.Plugins = append(result.Plugins, *dep)
	}
	createDepFileWithContent(workDir, result)
	return workDir
}

func createDepFileWithContent(workDir string, obj interface{}) {
	createFile(workDir, depFileName, 0665)
	bytes, err := yaml.Marshal(obj)
	if err != nil {
		log.Fatal(err)
	}
	writeContentBytes(workDir, depFileName, bytes)
}

func createDepFileWithIllegalContent(workDir string) {
	path := filepath.Join(workDir, depFileName)
	createFileByPath(path)
	writeContent(path, "illegal content")
}

func createFileWithContent(path string, content string) {
	createFileByPath(path)
	writeContent(path, content)
}

func toYamlQuiet(obj interface{}) string {
	fileContent, err := yaml.Marshal(obj)
	if err != nil {
		log.Fatal(err)
	}
	return string(fileContent)
}

func removeAll(path string) {
	err := os.RemoveAll(path)
	if err != nil {
		log.Println(err)
	}
}

func createTestWorkDir() string {
	dir, err := ioutil.TempDir("", "test")
	if err != nil {
		log.Fatal(err)
	}
	return dir
}

func createDir(parent string, name string) string {
	d := filepath.Join(parent, name)
	return createDirByPath(d)
}

func createDirByPath(path string) string {
	err := os.Mkdir(path, 0755)
	if err != nil {
		log.Fatal(err)
	}
	return path
}

func createFileByPath(path string) {
	to, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer to.Close()

	err = os.Chmod(path, 0655)
	if err != nil {
		log.Fatal(err)
	}
}

func createFile(parent string, name string, m os.FileMode) {
	path := filepath.Join(parent, name)
	to, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer to.Close()

	err = os.Chmod(path, m)
	if err != nil {
		log.Fatal(err)
	}
}

func writeContentBytes(parent string, name string, content []byte) {
	path := filepath.Join(parent, name)
	err := files.New().CreateFile(path, bytes.NewReader(content))
	if err != nil {
		log.Fatal(err)
	}
}

func writeContent(path string, content string) {
	err := files.New().CreateFile(path, strings.NewReader(content))
	if err != nil {
		log.Fatal(err)
	}
}
