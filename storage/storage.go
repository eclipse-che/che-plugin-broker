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
	"sync"

	"github.com/eclipse/che-plugin-broker/model"
)

// New creates new instance of storage
func New() Storage {
	return &storageImpl{}
}

type Storage interface {
	Plugins() ([]model.ChePlugin, error)
	AddPlugin(plugin model.ChePlugin) error
}

// Storage stores broker execution results
type storageImpl struct {
	sync.RWMutex
	plugins []model.ChePlugin
}

// Plugins returns configuration of Che Plugins resolved during the broker execution.
// At any particular point of time configuration might be incomplete if tooling resolution failed or not completed yet
func (s *storageImpl) Plugins() ([]model.ChePlugin, error) {
	s.Lock()
	defer s.Unlock()
	return s.plugins, nil
}

// AddPlugin adds configuration of model.ChePlugin to the results of broker execution
// by copying data from model.PluginMeta
func (s *storageImpl) AddPlugin(plugin model.ChePlugin) error {
	s.Lock()
	defer s.Unlock()
	s.plugins = append(s.plugins, plugin)
	return nil
}
