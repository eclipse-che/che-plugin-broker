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
	"sync"

	"github.com/eclipse/che-plugin-broker/model"
)

var s = &Storage{
	status: model.StatusIdle,
}

// Storage stores broker execution results
type Storage struct {
	sync.RWMutex
	status  model.BrokerStatus
	err     string
	tooling []model.ToolingConf
}

// Status returns current status of broker execution
func Status() model.BrokerStatus {
	s.Lock()
	defer s.Unlock()
	return s.status
}

// SetStatus sets current status of broker execution
func SetStatus(status model.BrokerStatus) (ok bool, currentValue model.BrokerStatus) {
	s.Lock()
	defer s.Unlock()
	switch {
	case s.status == model.StatusIdle && status == model.StatusStarting:
		fallthrough
	case s.status == model.StatusStarting && status == model.StatusDone:
		s.status = status
		return true, status
	default:
		return false, s.status
	}
}

// Err returns error message of broker execution if any
func Err() string {
	s.Lock()
	defer s.Unlock()
	return s.err
}

// SetErr sets error message of broker execution
func SetErr(err string) {
	s.Lock()
	defer s.Unlock()
	s.err = err
}

// Tooling returns configuration of Che plugins tooling resolved during the broker execution.
// At any particular point of time configuration might be incomplete if tooling resolution failed or not completed yet
func Tooling() (*[]model.ToolingConf, error) {
	s.Lock()
	defer s.Unlock()
	return &s.tooling, nil
}

// AddTooling adds configuration of model.ToolingConf to the results of broker execution
func AddTooling(tooling *model.ToolingConf) error {
	s.Lock()
	defer s.Unlock()
	s.tooling = append(s.tooling, *tooling)
	return nil
}
