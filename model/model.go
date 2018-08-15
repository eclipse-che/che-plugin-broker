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

package model

type BrokerStatus string

// Broker statuses
const (
	StatusIdle BrokerStatus = "IDLE"

	StatusStarting BrokerStatus = "STARTING"

	StatusDone BrokerStatus = "DONE"

	StatusFailed BrokerStatus = "FAILED"
)

type PluginMeta struct {
	ID string `json:"id" yaml:"id"`

	Name string `json:"name" yaml:"name"`

	Type string `json:"type" yaml:"type"`

	Description string `json:"description" yaml:"description"`

	Version string `json:"version" yaml:"version"`

	Title string `json:"title" yaml:"title"`

	Icon string `json:"icon" yaml:"icon"`

	URL string `json:"url" yaml:"url"`
}

type Endpoint struct {
	Name       string `json:"name" yaml:"name"`
	Public     bool   `json:"public" yaml:"public"`
	TargetPort int    `json:"targetPort yaml:"targetPort"`
}

type EnvVar struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value" yaml:"value"`
}

type EditorCommand struct {
	Name       string   `json:"name" yaml:"name"`
	WorkingDir string   `json:"working-dir" yaml:"working-dir"`
	Command    []string `json:"command" yaml:"command"`
}

type Volume struct {
	MountPath string `json:"mountPath" yaml:"mountPath"`
	Name      string `json:"name" yaml:"name"`
}

type ExposedPort struct {
	ExposedPort int `json:"exposedPort" yaml:"exposedPort"`
}

type Container struct {
	Name           string          `json:"name" yaml:"name"`
	Image          string          `json:"image" yaml:"image"`
	Env            []EnvVar        `json:"env" yaml:"env"`
	EditorCommands []EditorCommand `json:"editor-commands" yaml:"editor-commands"`
	Volumes        []Volume        `json:"volumes" yaml:"volumes"`
	Ports          []ExposedPort   `json:"ports" yaml:"ports"`
}

type Editor struct {
	ID      string   `json:"id" yaml:"id"`
	Plugins []string `json:"plugins" yaml:"plugins"`
}

type ToolingConf struct {
	Endpoints  []Endpoint  `json:"endpoints" yaml:"endpoints"`
	Containers []Container `json:"containers" yaml:"containers"`
	Editors    []Editor    `json:"editors" yaml:"editors"`
}

type CheDependency struct {
	ID       string `json:"id" yaml:"id"`
	Version  string `json:"version" yaml:"version"`
	Location string `json:"location" yaml:"location"`
	URL      string `json:"url" yaml:"url"`
}

type CheDependencies struct {
	Plugins []CheDependency `json:"plugins" yaml:"plugins"`
}
