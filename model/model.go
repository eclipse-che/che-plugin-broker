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

package model

// RuntimeID is an identifier of running workspace.
// Included to the plugin broker log events.
type RuntimeID struct {
	// Workspace is an identifier of the workspace e.g. "workspace123456".
	Workspace string `json:"workspaceId" yaml:"workspaceId"`

	// Environment is a name of environment e.g. "default".
	Environment string `json:"envName" yaml:"envName"`

	// OwnerId is an identifier of user who is runtime owner.
	OwnerId string `json:"ownerId" yaml:"ownerId"`
}

type PluginMeta struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`

	Spec PluginMetaSpec `json:"spec" yaml:"spec"`

	ID string `json:"id" yaml:"id"`

	Name string `json:"name" yaml:"name"`

	DisplayName string `json:"displayName" yaml:"displayName"`

	Publisher string `json:"publisher" yaml:"publisher"`

	Type string `json:"type" yaml:"type"`

	Description string `json:"description" yaml:"description"`

	Version string `json:"version" yaml:"version"`

	Title string `json:"title" yaml:"title"`

	Icon string `json:"icon" yaml:"icon"`
}

type PluginMetaSpec struct {
	Endpoints      []Endpoint  `json:"endpoints" yaml:"endpoints"`
	Containers     []Container `json:"containers" yaml:"containers"`
	InitContainers []Container `json:"initContainers" yaml:"initContainers"`
	WorkspaceEnv   []EnvVar    `json:"workspaceEnv" yaml:"workspaceEnv"`
	Extensions     []string    `json:"extensions" yaml:"extensions"`
}

type PluginFQN struct {
	Registry  string `json:"registry,omitempty" yaml:"registry,omitempty"`
	ID        string `json:"id" yaml:"id"`
	Reference string `json:"reference" yaml:"reference"`
}

type Endpoint struct {
	Name       string            `json:"name" yaml:"name"`
	Public     bool              `json:"public" yaml:"public"`
	TargetPort int               `json:"targetPort" yaml:"targetPort"`
	Attributes map[string]string `json:"attributes" yaml:"attributes"`
}

type EnvVar struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value" yaml:"value"`
}

type Command struct {
	Name       string   `json:"name" yaml:"name"`
	WorkingDir string   `json:"workingDir" yaml:"workingDir"`
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
	Name         string        `json:"name,omitempty" yaml:"name,omitempty"`
	Image        string        `json:"image,omitempty" yaml:"image,omitempty"`
	Env          []EnvVar      `json:"env" yaml:"env"`
	Commands     []Command     `json:"commands" yaml:"commands"`
	Volumes      []Volume      `json:"volumes" yaml:"volumes"`
	Ports        []ExposedPort `json:"ports" yaml:"ports"`
	MemoryLimit  string        `json:"memoryLimit,omitempty" yaml:"memoryLimit,omitempty"`
	MountSources bool          `json:"mountSources" yaml:"mountSources"`
}

type ChePlugin struct {
	ID             string      `json:"id" yaml:"id"`
	Version        string      `json:"version" yaml:"version"`
	Name           string      `json:"name" yaml:"name"`
	Publisher      string      `json:"publisher" yaml:"publisher"`
	Endpoints      []Endpoint  `json:"endpoints" yaml:"endpoints"`
	Containers     []Container `json:"containers" yaml:"containers"`
	InitContainers []Container `json:"initContainers" yaml:"initContainers"`
	WorkspaceEnv   []EnvVar    `json:"workspaceEnv" yaml:"workspaceEnv"`
}
