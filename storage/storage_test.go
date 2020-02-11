//
// Copyright (c) 2018-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/eclipse/che-plugin-broker/model"
)

func TestAddingPluginToStorage(t *testing.T) {
	var s = &storageImpl{}
	plugin := model.ChePlugin{
		ID:           "pub/plugin/vers",
		Version:      "1.0.0",
		Publisher:    "test publisher",
		Name:         "test-plugin",
		Containers:   []model.Container{{Name: "container"}},
		Endpoints:    []model.Endpoint{{Name: "endpoint"}},
		WorkspaceEnv: []model.EnvVar{{Name: "wsEnv1", Value: "wsEnvValue1"}},
	}

	if err := s.AddPlugin(plugin); err != nil {
		t.Errorf("Adding plugin failed with error: %s", err)
	}

	actualNumber := len(s.plugins)
	if actualNumber != 1 {
		t.Errorf("Storage has %v elements after adding one plugin", actualNumber)
	}

	actual := s.plugins[0]

	assert.Equal(t, actual, plugin)
}

func TestAddingDuplicatePluginToStorage(t *testing.T) {
	var s = &storageImpl{}
	plugin := model.ChePlugin{
		ID:           "pub/plugin/vers",
		Version:      "1.0.0",
		Publisher:    "test publisher",
		Name:         "test-plugin",
		Containers:   []model.Container{{Name: "container"}},
		Endpoints:    []model.Endpoint{{Name: "endpoint"}},
		WorkspaceEnv: []model.EnvVar{{Name: "wsEnv1", Value: "wsEnvValue1"}},
	}

	if err := s.AddPlugin(plugin); err != nil {
		t.Errorf("Adding plugin failed with error: %s", err)
	}
	if err := s.AddPlugin(plugin); err != nil {
		t.Errorf("Adding plugin failed with error: %s", err)
	}

	actualNumber := len(s.plugins)
	if actualNumber != 2 {
		t.Errorf("Storage has %v elements after adding one plugin", actualNumber)
	}

	assert.Equal(t, s.plugins[0], plugin)
	assert.Equal(t, s.plugins[1], plugin)
}

func TestGettingPluginsFromStorage(t *testing.T) {
	var s = &storageImpl{}
	s.plugins = []model.ChePlugin{
		{
			ID:           "pub/plugin/1.0.0",
			Version:      "1.0.0",
			Name:         "plugin",
			Publisher:    "pub",
			Containers:   []model.Container{{Name: "container"}},
			Endpoints:    []model.Endpoint{{Name: "endpoint"}},
			WorkspaceEnv: []model.EnvVar{{Name: "wsEnv1", Value: "wsEnvValue1"}},
		},
		{
			ID:           "pub2/plugin2/v1.1.0",
			Version:      "v1.1.0",
			Name:         "plugin2",
			Publisher:    "pub2",
			Containers:   []model.Container{{Name: "container2"}},
			Endpoints:    []model.Endpoint{{Name: "endpoint2"}},
			WorkspaceEnv: []model.EnvVar{{Name: "wsEnv2", Value: "wsEnvValue2"}},
		},
	}

	chePlugins, e := s.Plugins()

	if e != nil {
		t.Errorf("Error occurs during toolling receiving: %s", e)
	}

	assert.ElementsMatch(t, s.plugins, chePlugins, "Plugins list is not expected")
}

func TestGettingPluginsFromStorageWhenNoPluginIsAdded(t *testing.T) {
	var s = &storageImpl{}

	actual, e := s.Plugins()

	if e != nil {
		t.Errorf("Error occurs during toolling receiving: %s", e)
	}

	assert.True(t, len(actual) == 0)
}

func TestNewCreatesEmptyStorage(t *testing.T) {
	actual := New()
	expected := &storageImpl{}

	assert.Equal(t, actual, expected)
}
