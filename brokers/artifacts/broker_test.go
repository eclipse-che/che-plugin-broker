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

package artifacts

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"testing"

	commonMock "github.com/eclipse/che-plugin-broker/common/mocks"
	"github.com/eclipse/che-plugin-broker/model"
	utilMock "github.com/eclipse/che-plugin-broker/utils/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v2"
)

type mocks struct {
	commonBroker *commonMock.Broker
	ioUtils      *utilMock.IoUtil
	rand         *commonMock.Random
	broker       *Broker
}

func initMocks() *mocks {
	commonBroker := &commonMock.Broker{}

	ioUtils := &utilMock.IoUtil{}
	rand := &commonMock.Random{}

	commonBroker.On("PrintInfo", mock.AnythingOfType("string"))
	// It doesn't seem to be possible to mock variadic arguments
	commonBroker.On("PrintInfo", mock.AnythingOfType("string"), mock.Anything)
	commonBroker.On("PrintInfo", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything)
	commonBroker.On("PrintDebug", mock.AnythingOfType("string"))
	commonBroker.On("PrintDebug", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything)
	commonBroker.On("PubFailed", mock.AnythingOfType("string"))
	commonBroker.On("PubLog", mock.AnythingOfType("string"))
	commonBroker.On("PubStarted")
	commonBroker.On("PrintPlan", mock.AnythingOfType("[]model.PluginMeta"))
	commonBroker.On("CloseConsumers")
	commonBroker.On("PubDone", mock.AnythingOfType("string"))

	return &mocks{
		commonBroker: commonBroker,
		ioUtils:      ioUtils,
		rand:         rand,
		broker: &Broker{
			Broker:  commonBroker,
			ioUtils: ioUtils,
			rand:    rand,
		},
	}
}

func TestStartCleansPluginDirectoryOnStart(t *testing.T) {
	m := initMocks()
	m.ioUtils.On("GetFilesByGlob", mock.AnythingOfType("string")).Return([]string{"testPath1", "testPath2"}, nil)
	m.ioUtils.On("RemoveAll", "testPath1").Return(nil)
	m.ioUtils.On("RemoveAll", "testPath2").Return(nil)

	err := m.broker.Start([]model.PluginFQN{}, "default.io")
	assert.Nil(t, err)
	m.ioUtils.AssertExpectations(t)
	m.commonBroker.AssertCalled(t, "CloseConsumers")
}

func TestStartLogsErrorOnFailureToGlob(t *testing.T) {
	expectedError := errors.New("failed")

	m := initMocks()
	m.ioUtils.On("GetFilesByGlob", mock.AnythingOfType("string")).Return(nil, expectedError)
	m.commonBroker.On("PrintInfo", "WARN: failed to clear /plugins directory. Error: %s", expectedError)

	err := m.broker.Start([]model.PluginFQN{}, "default.io")
	assert.Nil(t, err)
	m.ioUtils.AssertExpectations(t)
	m.commonBroker.AssertCalled(t, "PrintInfo", "WARN: failed to clear /plugins directory. Error: %s", expectedError)
	m.commonBroker.AssertCalled(t, "CloseConsumers")
}

func TestStartLogsErrorOnFailureToRemoveAll(t *testing.T) {
	expectedError := errors.New("failed")

	m := initMocks()
	m.ioUtils.On("GetFilesByGlob", mock.AnythingOfType("string")).Return([]string{"testPath1", "testPath2"}, nil)
	m.ioUtils.On("RemoveAll", "testPath1").Return(expectedError)
	m.ioUtils.On("RemoveAll", "testPath2").Return(nil)
	m.commonBroker.On("PrintInfo", "WARN: failed to remove '%s'. Error: %s", "testPath1", expectedError)

	err := m.broker.Start([]model.PluginFQN{}, "default.io")
	assert.Nil(t, err)

	m.ioUtils.AssertExpectations(t)
	m.commonBroker.AssertCalled(t, "PrintInfo", "WARN: failed to remove '%s'. Error: %s", "testPath1", expectedError)
	m.commonBroker.AssertCalled(t, "CloseConsumers")
}

func TestStartFailsIfFetchFails(t *testing.T) {
	expectedError := errors.New("testError")
	expectedErrorString := fmt.Sprintf("Failed to download plugin meta: failed to fetch plugin meta.yaml from URL 'test/plugins/test/meta.yaml': %s", expectedError)
	pluginFQNs := []model.PluginFQN{
		generatePluginFQN("test", "test", ""),
	}

	m := initMocks()
	m.ioUtils.On("GetFilesByGlob", mock.AnythingOfType("string")).Return([]string{}, nil)
	m.ioUtils.On("Fetch", mock.AnythingOfType("string")).Return(nil, expectedError)

	err := m.broker.Start(pluginFQNs, "default.io")
	assert.EqualError(t, err, expectedErrorString)
	m.commonBroker.AssertCalled(t, "PubFailed", expectedErrorString)
	m.commonBroker.AssertCalled(t, "PubLog", expectedErrorString)
}

func TestFailureResolvingRelativeExtensionPaths(t *testing.T) {
	_, pluginMetaBytes := loadPluginMetaFromFile(t, "vscode-java-0.50.0-relative.yaml")
	defaultRegistry := ""
	pluginFQNs := []model.PluginFQN{
		generatePluginFQN("testRegistry", "testID", ""),
	}
	expectedErrorString := "cannot resolve relative extension path without default registry"

	m := initMocks()
	m.ioUtils.On("GetFilesByGlob", mock.AnythingOfType("string")).Return([]string{}, nil)
	m.ioUtils.On("Fetch", mock.AnythingOfType("string")).Return(pluginMetaBytes, nil)

	err := m.broker.Start(pluginFQNs, defaultRegistry)
	assert.EqualError(t, err, expectedErrorString)
	m.commonBroker.AssertCalled(t, "PubFailed", expectedErrorString)
	m.commonBroker.AssertCalled(t, "PubLog", expectedErrorString)
	m.commonBroker.AssertCalled(t, "CloseConsumers")
}

func TestStartPropagatesErrorOnPluginProcessing(t *testing.T) {
	pluginMeta, _ := loadPluginMetaFromFile(t, "vscode-java-0.50.0.yaml")
	pluginMeta.Spec.Extensions = []string{}
	var pluginMetaBytes []byte
	pluginMetaBytes, yamlErr := yaml.Marshal(pluginMeta)
	if yamlErr != nil {
		t.Errorf("Failed to marshal yaml")
	}
	defaultRegistry := ""
	pluginFQNs := []model.PluginFQN{
		generatePluginFQN("testRegistry", "testID", ""),
	}
	expectedErrorString := fmt.Sprintf("Field 'extensions' is not found in the description of the plugin '%s'", "testID")

	m := initMocks()
	m.ioUtils.On("GetFilesByGlob", mock.AnythingOfType("string")).Return([]string{}, nil)
	m.ioUtils.On("Fetch", mock.AnythingOfType("string")).Return(pluginMetaBytes, nil)

	err := m.broker.Start(pluginFQNs, defaultRegistry)
	assert.EqualError(t, err, expectedErrorString)
	m.commonBroker.AssertCalled(t, "PubFailed", expectedErrorString)
	m.commonBroker.AssertCalled(t, "PubLog", expectedErrorString)
	m.commonBroker.AssertCalled(t, "CloseConsumers")
}

func TestStartSuccessfulFlow(t *testing.T) {
	m := initMocks()

	defaultRegistry := "default.io"
	pluginFQNs := []model.PluginFQN{
		generatePluginFQN("testRegistry", "testID", ""),
	}
	m.ioUtils.On("GetFilesByGlob", mock.AnythingOfType("string")).Return([]string{}, nil)
	m.ioUtils.On("Fetch", "testRegistry/plugins/testID/meta.yaml").Return([]byte{}, nil)

	err := m.broker.Start(pluginFQNs, defaultRegistry)
	assert.Nil(t, err)

	m.commonBroker.AssertCalled(t, "PubStarted")
	m.commonBroker.AssertCalled(t, "PrintInfo", "Downloading plugin extensions")
	m.commonBroker.AssertCalled(t, "PrintInfo", "All plugin artifacts have been successfully downloaded")
	m.commonBroker.AssertCalled(t, "PubDone", "")
	m.commonBroker.AssertCalled(t, "CloseConsumers")
}

func TestProcessPluginDoesNothingForChePlugin(t *testing.T) {
	meta, _ := loadPluginMetaFromFile(t, "machine-exec-7.4.0.yaml")
	m := initMocks()
	err := m.broker.ProcessPlugin(meta)
	assert.Nil(t, err)
}

func TestProcessPluginReturnsErrorWhenNoExtensions(t *testing.T) {
	meta, _ := loadPluginMetaFromFile(t, "vscode-java-0.50.0.yaml")
	meta.Spec.Extensions = []string{}
	meta.ID = "testId"

	expectedError := fmt.Sprintf(errorNoExtFieldsTemplate, "testId")

	m := initMocks()
	err := m.broker.ProcessPlugin(meta)
	assert.EqualError(t, err, expectedError)
}

func TestProcessPluginReturnsErrorWhenFailureToCreateTempDir(t *testing.T) {
	meta, _ := loadPluginMetaFromFile(t, "vscode-java-0.50.0.yaml")
	expectedError := errors.New("test error")

	m := initMocks()
	m.ioUtils.On("TempDir", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return("", expectedError)

	err := m.broker.ProcessPlugin(meta)
	assert.EqualError(t, err, expectedError.Error())
}

func TestProcessPluginReturnsErrorWhenFailureDownloadArchives(t *testing.T) {
	meta, _ := loadPluginMetaFromFile(t, "vscode-java-0.50.0.yaml")
	expectedError := errors.New("test error")
	expectedErrorRegexp := regexp.MustCompile("failed to download plugin from .*: test error")

	m := initMocks()
	m.ioUtils.On("TempDir", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return("testDir", nil)
	m.ioUtils.On("ResolveDestPathFromURL", mock.AnythingOfType("string"), "testDir").Return("testDestDir")
	m.ioUtils.On("Download", mock.AnythingOfType("string"), "testDestDir", true).Return("", expectedError)

	err := m.broker.ProcessPlugin(meta)
	assert.Regexp(t, expectedErrorRegexp, err.Error())
}

func TestProcessPluginReturnsErrorWhenCantMakeDir(t *testing.T) {
	meta, _ := loadPluginMetaFromFile(t, "vscode-java-0.50.0.yaml")
	expectedError := errors.New("test error")

	m := initMocks()
	m.ioUtils.On("TempDir", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return("testDir", nil)
	m.ioUtils.On("ResolveDestPathFromURL", mock.AnythingOfType("string"), "testDir").Return("testDestDir")
	m.ioUtils.On("Download", mock.AnythingOfType("string"), "testDestDir", true).Return("testDownloadDir", nil)
	m.ioUtils.On("MkDir", mock.AnythingOfType("string")).Return(expectedError)

	err := m.broker.ProcessPlugin(meta)
	assert.EqualError(t, err, expectedError.Error())
}

func TestProcessPluginReturnsErrorWhenCantCopyFile(t *testing.T) {
	meta, _ := loadPluginMetaFromFile(t, "vscode-java-0.50.0.yaml")
	expectedError := errors.New("test error")

	m := initMocks()
	m.ioUtils.On("TempDir", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return("testDir", nil)
	m.ioUtils.On("ResolveDestPathFromURL", mock.AnythingOfType("string"), "testDir").Return("testDestDir")
	m.ioUtils.On("Download", mock.AnythingOfType("string"), "testDestDir", true).Return("testDownloadDir", nil)
	m.ioUtils.On("MkDir", mock.AnythingOfType("string")).Return(nil)
	m.rand.On("String", 10).Return("xxxxx")
	m.ioUtils.On("CopyFile", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(expectedError)

	err := m.broker.ProcessPlugin(meta)
	assert.EqualError(t, err, expectedError.Error())
}

func TestProcessPluginProcessesAllExtensions(t *testing.T) {
	meta, _ := loadPluginMetaFromFile(t, "vscode-java-0.50.0.yaml")

	m := initMocks()
	m.ioUtils.On("TempDir", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return("testDir", nil)
	m.ioUtils.On("ResolveDestPathFromURL", mock.AnythingOfType("string"), "testDir").Return("testDestDir")
	m.ioUtils.On("Download", mock.AnythingOfType("string"), "testDestDir", true).Return("testDownloadDir", nil)
	m.ioUtils.On("MkDir", mock.AnythingOfType("string")).Return(nil)
	m.rand.On("String", 10).Return("xxxxx")
	m.ioUtils.On("CopyFile", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	err := m.broker.ProcessPlugin(meta)
	assert.Nil(t, err)
	m.ioUtils.AssertNumberOfCalls(t, "Download", 2)
	m.ioUtils.AssertNumberOfCalls(t, "CopyFile", 2)
}

func loadPluginMetaFromFile(t *testing.T, filename string) (model.PluginMeta, []byte) {
	path := filepath.Join("../testdata", filename)
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var pluginMeta model.PluginMeta
	if err := yaml.Unmarshal(bytes, &pluginMeta); err != nil {
		t.Fatal(err)
	}
	return pluginMeta, bytes
}

func generatePluginFQN(registry, id, reference string) model.PluginFQN {
	return model.PluginFQN{
		Registry:  registry,
		ID:        id,
		Reference: reference,
	}
}
