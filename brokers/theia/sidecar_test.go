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

package theia

import (
	"strconv"
	"testing"

	"github.com/eclipse/che-plugin-broker/common/mocks"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/stretchr/testify/assert"
)

func TestGenerateSidecar(t *testing.T) {
	testImage := "test/test:latest"
	random6 := "123456"
	random10 := "1234567890"
	randomInt := 8889
	rand := &mocks.Random{}
	rand.On("String", 6).Return(random6)
	rand.On("String", 10).Return(random10)
	rand.On("IntFromRange", 4000, 10000).Return(randomInt)

	expected := generateTestToolingWithVars(testImage, random6, randomInt, random10)

	actual := GenerateSidecar(testImage, rand)

	assert.Equal(t, expected, actual)
}

func TestAddExtension(t *testing.T) {
	type args struct {
		toolingConf *model.ToolingConf
		pj          model.PackageJSON
	}
	tests := []struct {
		name string
		args args
		want *model.ToolingConf
	}{
		{
			name: "Test adding extension with package.json data with a-zA-Z0-9_ symbols",
			args: args{
				toolingConf: generateTestTooling(),
				pj:          generatePackageJSON("pluginName8", "publisherName1_0_"),
			},
			want: generateTestToolingWithExtension("pluginName8", "publisherName1_0_"),
		},
		{
			name: "Test adding extension with package.json data with # symbol",
			args: args{
				toolingConf: generateTestTooling(),
				pj:          generatePackageJSON("plugin#Name8", "publisherName1_0_"),
			},
			want: generateTestToolingWithExtension("plugin_Name8", "publisherName1_0_"),
		},
		{
			name: "Test adding extension with package.json data with @ symbol",
			args: args{
				toolingConf: generateTestTooling(),
				pj:          generatePackageJSON("plu@ginName8", "publisherName1_0_"),
			},
			want: generateTestToolingWithExtension("plu_ginName8", "publisherName1_0_"),
		},
		{
			name: "Test adding extension with package.json data with : symbol",
			args: args{
				toolingConf: generateTestTooling(),
				pj:          generatePackageJSON("pluginName8", "publisherName:1_0_"),
			},
			want: generateTestToolingWithExtension("pluginName8", "publisherName_1_0_"),
		},
		{
			name: "Test adding extension with package.json data with ? symbol",
			args: args{
				toolingConf: generateTestTooling(),
				pj:          generatePackageJSON("pluginName8", "publisherName?1_0_"),
			},
			want: generateTestToolingWithExtension("pluginName8", "publisherName_1_0_"),
		},
		{
			name: "Test adding extension with package.json data with - symbol",
			args: args{
				toolingConf: generateTestTooling(),
				pj:          generatePackageJSON("plugin-Name-8", "publisherName1_0_"),
			},
			want: generateTestToolingWithExtension("plugin_Name_8", "publisherName1_0_"),
		},
		{
			name: "Test adding extension with package.json data with ! symbol",
			args: args{
				toolingConf: generateTestTooling(),
				pj:          generatePackageJSON("plugin!Name8", "publisherName1_0_!"),
			},
			want: generateTestToolingWithExtension("plugin_Name8", "publisherName1_0__"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddExtension(tt.args.toolingConf, tt.args.pj)
		})
		assert.Equal(t, tt.args.toolingConf, tt.want)
	}
}

func generatePackageJSON(name string, publisher string) model.PackageJSON {
	return model.PackageJSON{
		Name:      name,
		Publisher: publisher,
	}
}

func generateTestToolingWithExtension(extName string, extPublisher string) *model.ToolingConf {
	testImage := "test/test:latest"
	random6 := "123456"
	random10 := "1234567890"
	randomInt := 8889
	tooling := generateTestToolingWithVars(testImage, random6, randomInt, random10)
	tooling.WorkspaceEnv = append(tooling.WorkspaceEnv, model.EnvVar{
		Name:  "THEIA_PLUGIN_REMOTE_ENDPOINT_" + extPublisher + "_" + extName,
		Value: "ws://" + random10 + ":" + strconv.Itoa(randomInt),
	})
	return tooling
}

func generateTestTooling() *model.ToolingConf {
	testImage := "test/test:latest"
	random6 := "123456"
	random10 := "1234567890"
	randomInt := 8889
	return generateTestToolingWithVars(testImage, random6, randomInt, random10)
}

func generateTestToolingWithVars(testImage string, nameSuffix string, port int, endpointName string) *model.ToolingConf {
	return &model.ToolingConf{
		Containers: []model.Container{
			{
				Name:  "pluginsidecar" + nameSuffix,
				Image: testImage,
				Volumes: []model.Volume{
					{
						Name:      "projects",
						MountPath: "/projects",
					},
					{
						Name:      "plugins",
						MountPath: "/plugins",
					},
				},
				Ports: []model.ExposedPort{
					{
						ExposedPort: port,
					},
				},
				Env: []model.EnvVar{
					{
						Name:  "THEIA_PLUGIN_ENDPOINT_PORT",
						Value: strconv.Itoa(port),
					},
				},
			},
		},
		Endpoints: []model.Endpoint{
			{
				Name:       endpointName,
				Public:     false,
				TargetPort: port,
			},
		},
	}
}
