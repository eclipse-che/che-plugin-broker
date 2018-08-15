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

type Storage struct {
	sync.RWMutex
	status  model.BrokerStatus
	err     string
	logs    []string
	tooling []model.ToolingConf
}

func Status() model.BrokerStatus {
	s.Lock()
	defer s.Unlock()
	return s.status
}

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

func Err() string {
	s.Lock()
	defer s.Unlock()
	return s.err
}

func SetErr(err string) {
	s.Lock()
	defer s.Unlock()
	s.err = err
}

func Logs() *[]string {
	s.Lock()
	defer s.Unlock()
	logsCopy := []string{}
	copy(logsCopy, s.logs)
	return &logsCopy
}

func AppendLogs(log string) {
	s.Lock()
	defer s.Unlock()
	s.logs = append(s.logs, log)
}

func Tooling() (*[]model.ToolingConf, error) {
	s.Lock()
	defer s.Unlock()
	return &s.tooling, nil
}

func AddTooling(tooling *model.ToolingConf) error {
	s.Lock()
	defer s.Unlock()
	s.tooling = append(s.tooling, *tooling)
	return nil
}
