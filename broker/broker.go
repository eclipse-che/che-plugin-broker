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

package broker

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
	yaml "gopkg.in/yaml.v2"
)

func ProcessPlugins(metas []model.PluginMeta) {
	for _, meta := range metas {
		err := processPlugin(meta)
		if err != nil {
			log.Panic(err)
		}
	}

	log.Println("Set success status")
	if ok, status := storage.SetStatus(model.StatusDone); !ok {
		log.Panicf("Setting '%s' broker status failed. Broker has '%s' state", model.StatusDone, status)
	}
}

func processPlugin(meta model.PluginMeta) error {
	url := meta.URL

	workDir, err := ioutil.TempDir("", "che-plugin-broker")
	if err != nil {
		return err
	}
	log.Printf("Working directory: %s", workDir)

	archivePath := filepath.Join(workDir, "testArchive.tar.gz")
	pluginPath := filepath.Join(workDir, "testArchive")

	// Download an archive
	log.Println("Downloading")
	err = download(url, archivePath)
	if err != nil {
		return err
	}

	log.Println("Untarring")
	// Untar it
	err = untar(archivePath, pluginPath)
	if err != nil {
		return err
	}

	log.Println("Resolving yamls")
	err = resolveToolingConfig(pluginPath)
	if err != nil {
		return err
	}

	log.Println("Copying dependencies")
	if err = copyDependencies(pluginPath); err != nil {
		return err
	}

	return nil
}

func resolveToolingConfig(workDir string) error {
	toolingConfPath := filepath.Join(workDir, "che-plugin.yaml")
	f, err := ioutil.ReadFile(toolingConfPath)
	if err != nil {
		return err
	}

	tooling := &model.ToolingConf{}
	if err := yaml.Unmarshal(f, tooling); err != nil {
		return err
	}

	return storage.AddTooling(tooling)
}

func copyDependencies(workDir string) error {
	depsConfPath := filepath.Join(workDir, "che-dependency.yaml")
	if _, err := os.Stat(depsConfPath); os.IsNotExist(err) {
		return nil
	}

	f, err := ioutil.ReadFile(depsConfPath)
	if err != nil {
		return err
	}

	deps := &model.CheDependencies{}
	if err := yaml.Unmarshal(f, deps); err != nil {
		return err
	}

	for _, dep := range deps.Plugins {
		switch {
		case dep.Location != "" && dep.URL != "":
			m := fmt.Sprintf("Plugin dependency '%s:%s' contains both 'location' and 'url' fields while just one should be present", dep.ID, dep.Version)
			return errors.New(m)
		case dep.Location != "":
			fileDest := resolveDestPath(dep.Location, "/plugins")
			if err = copyFile(filepath.Join(workDir, dep.Location), fileDest); err != nil {
				return err
			}
		case dep.URL != "":
			fileDest := resolveDestPathFromURL(dep.URL, "/plugins")
			if err = download(dep.URL, fileDest); err != nil {
				return err
			}
		default:
			m := fmt.Sprintf("Plugin dependency '%s:%s' contains neither 'location' nor 'url' field", dep.ID, dep.Version)
			return errors.New(m)
		}
	}

	return nil
}
