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

package storage

import (
	"sync"

	"github.com/eclipse/che-plugin-broker/model"
)

// New creates new instance of storage
func New() *Storage {
	return &Storage{
		status: model.StatusIdle,
	}
}

// Storage stores broker execution results
type Storage struct {
	sync.RWMutex
	status  model.BrokerStatus
	plugins []model.ChePlugin
}

// Status returns current status of broker execution
func (s *Storage) Status() model.BrokerStatus {
	s.Lock()
	defer s.Unlock()
	return s.status
}

// SetStatus sets current status of broker execution
func (s *Storage) SetStatus(status model.BrokerStatus) (ok bool, currentValue model.BrokerStatus) {
	s.Lock()
	defer s.Unlock()
	switch {
	case s.status == model.StatusIdle && status == model.StatusStarted:
		fallthrough
	case s.status == model.StatusStarted && status == model.StatusDone:
		s.status = status
		return true, status
	default:
		return false, s.status
	}
}

// Plugins returns configuration of Che Plugins resolved during the broker execution.
// At any particular point of time configuration might be incomplete if tooling resolution failed or not completed yet
func (s *Storage) Plugins() (*[]model.ChePlugin, error) {
	s.Lock()
	defer s.Unlock()
	return &s.plugins, nil
}

// AddPlugin adds configuration of model.ChePlugin to the results of broker execution
// by combining model.ToolingConf and model.PluginMeta
func (s *Storage) AddPlugin(meta *model.PluginMeta, tooling *model.ToolingConf) error {
	s.Lock()
	defer s.Unlock()
	plugin := &model.ChePlugin{
		ID:           meta.ID,
		Version:      meta.Version,
		Containers:   tooling.Containers,
		Editors:      tooling.Editors,
		Endpoints:    tooling.Endpoints,
		WorkspaceEnv: tooling.WorkspaceEnv,
	}
	s.plugins = append(s.plugins, *plugin)
	return nil
}
