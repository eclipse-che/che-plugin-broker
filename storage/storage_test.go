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

package storage

import (
	"testing"

	"github.com/eclipse/che-plugin-broker/model"
	"github.com/stretchr/testify/assert"
)

func TestSettingStatusOfStorage(t *testing.T) {
	tables := []struct {
		initialStatus model.BrokerStatus
		newStatus     model.BrokerStatus
		isChanged     bool
	}{
		{model.StatusIdle, model.StatusStarted, true},
		{model.StatusIdle, model.StatusDone, false},
		{model.StatusIdle, model.StatusFailed, false},
		{model.StatusIdle, model.StatusIdle, false},
		{model.StatusStarted, model.StatusDone, true},
		{model.StatusStarted, model.StatusIdle, false},
		{model.StatusStarted, model.StatusFailed, false},
		{model.StatusStarted, model.StatusStarted, false},
		{model.StatusDone, model.StatusIdle, false},
		{model.StatusDone, model.StatusDone, false},
		{model.StatusDone, model.StatusStarted, false},
		{model.StatusDone, model.StatusDone, false},
	}

	for _, table := range tables {
		//storage1 := &Storage{status:table.initialStatus}
		s.status = table.initialStatus
		ok, currentValue := SetStatus(table.newStatus)

		if ok != table.isChanged {
			t.Errorf("Status expected not to be changed from %s to %s but it was", table.initialStatus, table.newStatus)
		}

		if table.initialStatus != table.newStatus && !ok && currentValue != table.initialStatus {
			t.Errorf("Current state is changed from %s to %s but it shouldn't be", table.initialStatus, currentValue)
		}
	}
}

func TestAddingTollingToStorage(t *testing.T) {
	meta := model.PluginMeta{
		ID:      "org.plugin.id",
		Version: "1.0.0",
		Name:    "test-plugin",
	}
	conf := model.ToolingConf{
		Containers: []model.Container{{Name: "container"}},
		Editors:    []model.Editor{{ID: "editor-id"}},
		Endpoints:  []model.Endpoint{{Name: "endpoint"}},
	}

	AddPlugin(&meta, &conf)

	actualNumber := len(s.plugins)
	if actualNumber != 1 {
		t.Errorf("Storage has %v elements after adding one plugin", actualNumber)
	}

	plugin := s.plugins[0]

	assert.Equal(t, meta.ID, plugin.ID, "Plugin ID is not expected")
	assert.Equal(t, meta.Version, plugin.Version, "Plugin Version is not expected")

	assert.ElementsMatch(t, conf.Containers, plugin.Containers, "Plugin Containers are not expected")
	assert.ElementsMatch(t, conf.Endpoints, plugin.Endpoints, "Plugin Endpoints are not expected")
	assert.ElementsMatch(t, conf.Editors, plugin.Editors, "Plugin Editors are not expected")
}

func TestGettingTollingFromStorage(t *testing.T) {
	s.plugins = []model.ChePlugin{
		{
			ID:         "org.plugin.id",
			Version:    "1.0.0",
			Containers: []model.Container{{Name: "container"}},
			Editors:    []model.Editor{{ID: "editor-id"}},
			Endpoints:  []model.Endpoint{{Name: "endpoint"}},
		},
	}

	chePlugins, e := Plugins()

	if e != nil {
		t.Errorf("Error occurs during toolling receiving: %s", e)
	}

	assert.ElementsMatch(t, s.plugins, *chePlugins, "Plugins list is not expected")
}
