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

package vscode

type marketplaceResponse struct {
	Results []result
}

type result struct {
	Extensions []extension
}

type extension struct {
	Versions []version
}

type version struct {
	Version string
	Files   []file
}

type file struct {
	AssetType string
	Source    string
}

// PackageJSON represents package.json file of JS based projects
type PackageJSON struct {
	Name      string `json:"name" yaml:"name"`
	Publisher string `json:"publisher" yaml:"publisher"`
}
