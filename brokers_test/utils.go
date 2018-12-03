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

package brokers_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/eclipse/che-plugin-broker/files"
)

func CreateTestWorkDir() string {
	dir, err := ioutil.TempDir("", "test")
	if err != nil {
		log.Fatal(err)
	}
	return dir
}

func CreateDirByPath(path string) string {
	err := os.Mkdir(path, 0755)
	if err != nil {
		log.Fatal(err)
	}
	return path
}

func CreateDir(parent string, name string) string {
	d := filepath.Join(parent, name)
	return CreateDirByPath(d)
}

func CreateFileByPath(path string) {
	to, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer to.Close()

	err = os.Chmod(path, 0655)
	if err != nil {
		log.Fatal(err)
	}
}

func CreateFile(parent string, name string, m os.FileMode) {
	path := filepath.Join(parent, name)
	to, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer to.Close()

	err = os.Chmod(path, m)
	if err != nil {
		log.Fatal(err)
	}
}

func CreateFileWithContent(path string, content string) {
	CreateFileByPath(path)
	WriteContent(path, content)
}

func RemoveAll(path string) {
	err := os.RemoveAll(path)
	if err != nil {
		log.Println(err)
	}
}

func WriteContentBytes(parent string, name string, content []byte) {
	path := filepath.Join(parent, name)
	err := files.New().CreateFile(path, bytes.NewReader(content))
	if err != nil {
		log.Fatal(err)
	}
}

func WriteContent(path string, content string) {
	err := files.New().CreateFile(path, strings.NewReader(content))
	if err != nil {
		log.Fatal(err)
	}
}

func ToYamlQuiet(obj interface{}) string {
	fileContent, err := yaml.Marshal(obj)
	if err != nil {
		log.Fatal(err)
	}
	return string(fileContent)
}

func ToJSONQuiet(obj interface{}) string {
	fileContent, err := json.Marshal(obj)
	if err != nil {
		log.Fatal(err)
	}
	return string(fileContent)
}
