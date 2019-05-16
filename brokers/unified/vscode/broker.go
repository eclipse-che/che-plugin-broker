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

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	jsonrpc "github.com/eclipse/che-go-jsonrpc"
	"github.com/eclipse/che-plugin-broker/cfg"
	"github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
	"github.com/eclipse/che-plugin-broker/utils"
)

const marketplace = "https://marketplace.visualstudio.com/_apis/public/gallery/extensionquery"
const bodyFmt = `{"filters":[{"criteria":[{"filterType":7,"value":"%s"}],"pageNumber":1,"pageSize":1,"sortBy":0, "sortOrder":0 }],"assetTypes":["Microsoft.VisualStudio.Services.VSIXPackage"],"flags":131}`
const assetType = "Microsoft.VisualStudio.Services.VSIXPackage"
const errorNoExtFieldsTemplate = "Field 'extensions' is not found in the description of the plugin '%s'"
const vsixManifestFileName = "extension.vsixmanifest"
const vsixPackageJSONFolderName = "extension"

var re = regexp.MustCompile(`[^a-zA-Z_0-9]+`)

type brokerImpl struct {
	common.Broker
	ioUtil           utils.IoUtil
	Storage          storage.Storage
	client           *http.Client
	rand             common.Random
	localhostSidecar bool
}

// NewBrokerWithParams creates Che VS Code extension broker instance
func NewBrokerWithParams(
	broker common.Broker,
	ioUtil utils.IoUtil,
	storage storage.Storage,
	rand common.Random,
	httpClient *http.Client,
	localhostSidecar bool) common.BrokerImpl {
	return &brokerImpl{
		Broker:           broker,
		ioUtil:           ioUtil,
		rand:             rand,
		Storage:          storage,
		client:           httpClient,
		localhostSidecar: localhostSidecar,
	}
}

// Start executes plugins metas processing and sends data to Che master
func (b *brokerImpl) Start(metas []model.PluginMeta) {
	b.PubStarted()
	b.PrintInfo("Started VS Code Plugin Broker")

	b.PrintPlan(metas)

	b.PrintInfo("Starting VS Code extensions processing")
	for _, meta := range metas {
		err := b.ProcessPlugin(meta)
		if err != nil {
			b.PubFailed(err.Error())
			b.PrintFatal(err.Error())
		}
	}

	plugins, err := b.Storage.Plugins()
	if err != nil {
		b.PubFailed(err.Error())
		b.PrintFatal(err.Error())
	}
	pluginsBytes, err := json.Marshal(plugins)
	if err != nil {
		b.PubFailed(err.Error())
		b.PrintFatal(err.Error())
	}

	b.PrintInfo("All plugins have been successfully processed")
	result := string(pluginsBytes)
	b.PrintDebug(result)
	b.PubDone(result)
	b.CloseConsumers()
}

// PushEvents sets given tunnel as consumer of broker events.
func (b *brokerImpl) PushEvents(tun *jsonrpc.Tunnel) {
	b.Broker.PushEvents(tun, model.BrokerStatusEventType, model.BrokerResultEventType, model.BrokerLogEventType)
}

func (b *brokerImpl) ProcessPlugin(meta model.PluginMeta) error {
	b.PrintDebug("Started processing plugin '%s'", meta.ID)
	plugin := convertMetaToPlugin(meta)

	URLs, err := b.getBinariesURLs(meta)
	if err != nil {
		return err
	}

	workDir, err := b.ioUtil.TempDir("", "vscode-extension-broker")
	if err != nil {
		return err
	}

	archivesPaths, err := b.downloadArchives(URLs, meta, workDir)
	if err != nil {
		return err
	}

	// we only copy files for local plugin, so if local but copying is disabled do nothing
	if len(meta.Spec.Containers) == 0 && !cfg.OnlyApplyMetadataActions {
		err = b.injectLocalPlugin(plugin, archivesPaths)
		return err
	}
	return b.injectRemotePlugin(plugin, archivesPaths, workDir)
}

func (b *brokerImpl) injectLocalPlugin(plugin model.ChePlugin, archivesPaths []string) error {
	b.PrintDebug("Copying VS Code plugin '%s'", plugin.ID)
	for _, path := range archivesPaths {
		pluginName := b.generatePluginArchiveName(plugin)
		pluginPath := filepath.Join("/plugins", pluginName)
		b.PrintDebug("Copying VS Code extension archive from '%s' to '%s' for plugin '%s'", path, pluginPath, plugin.ID)
		err := b.ioUtil.CopyFile(path, pluginPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func getPluginUniqueName(plugin model.ChePlugin) string {
	return re.ReplaceAllString(plugin.Publisher+"_"+plugin.Name+"_"+plugin.Version, `_`)
}

func (b *brokerImpl) injectRemotePlugin(plugin model.ChePlugin, archivesPaths []string, workDir string) error {
	plugin = AddPluginRunnerRequirements(plugin, b.rand, b.localhostSidecar)
	for _, archive := range archivesPaths {
		if !cfg.OnlyApplyMetadataActions {
			pluginName := getPluginUniqueName(plugin)
			pluginFolderPath := filepath.Join("/sidecar-plugins", pluginName)
			err := b.ioUtil.MkDir(pluginFolderPath)
			if err != nil {
				return err
			}

			b.PrintDebug("Copying VS Code extension '%s' from '%s' to '%s'", plugin.ID, archive, pluginFolderPath)
			err = b.ioUtil.CopyResource(archive, pluginFolderPath + "/")
			if err != nil {
				return err
			}
		}
	}

	if !b.localhostSidecar {
		plugin = AddExtension(plugin)
	}

	return b.Storage.AddPlugin(plugin)
}

func convertMetaToPlugin(meta model.PluginMeta) model.ChePlugin {
	return model.ChePlugin{
		ID:           meta.ID,
		Name:         meta.Name,
		Publisher:    meta.Publisher,
		Version:      meta.Version,
		Containers:   meta.Spec.Containers,
		Endpoints:    meta.Spec.Endpoints,
		WorkspaceEnv: meta.Spec.WorkspaceEnv,
	}
}

func (b *brokerImpl) downloadArchives(URLs []string, meta model.PluginMeta, workDir string) ([]string, error) {
	paths := make([]string, 0)
	for _, URL := range URLs {
		archivePath := b.ioUtil.ResolveDestPathFromURL(URL, workDir)
		b.PrintDebug("Downloading VS Code extension archive '%s' for plugin '%s' to '%s'", URL, meta.ID, archivePath)
		b.PrintInfo("Downloading VS Code extension for plugin '%s'", meta.ID)
		err := b.downloadArchive(URL, archivePath)
		paths = append(paths, archivePath)
		if err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func (b *brokerImpl) getExtensionsAndURLs(meta model.PluginMeta) (e []string, u []string, err error) {
	extensions := make([]string, 0)
	URLs := make([]string, 0)

	if len(meta.Spec.Extensions) == 0 {
		return nil, nil, fmt.Errorf(errorNoExtFieldsTemplate, meta.ID)
	}
	for _, v := range meta.Spec.Extensions {
		ext, URL := extensionOrURL(v)
		switch {
		case ext != "":
			extensions = append(extensions, ext)
		case URL != "":
			URLs = append(URLs, URL)
		}
	}
	return extensions, URLs, nil
}

func (b *brokerImpl) getBinariesURLs(meta model.PluginMeta) ([]string, error) {
	extensions, URLs, err := b.getExtensionsAndURLs(meta)
	if err != nil {
		return nil, err
	}
	for _, ext := range extensions {
		URL, err := b.getExtensionArchiveURL(ext, meta)
		if err != nil {
			return nil, err
		}
		URLs = append(URLs, URL)
	}
	return URLs, nil
}

func extensionOrURL(extensionOrURL string) (extension string, URL string) {
	if strings.HasPrefix(extensionOrURL, "vscode:extension/") {
		return extensionOrURL, ""
	} else {
		return "", extensionOrURL
	}
}

func (b *brokerImpl) generatePluginArchiveName(plugin model.ChePlugin) string {
	return fmt.Sprintf("%s.%s.%s.%s", plugin.Publisher, plugin.Name, plugin.Version, b.rand.String(10))
}

func (b *brokerImpl) getExtensionArchiveURL(extension string, meta model.PluginMeta) (string, error) {
	response, err := b.fetchExtensionInfo(extension, meta)
	if err != nil {
		return "", err
	}

	URL, err := findAssetURL(response, meta)
	if err != nil {
		return "", err
	}
	return URL, nil
}

func (b *brokerImpl) downloadArchive(URL string, dest string) error {
	err := b.ioUtil.Download(URL, dest)
	retries := 5
	for i := 1; i <= retries && isRateLimitError(err); i++ {
		b.PrintInfo("VS Code marketplace access rate limit reached. Download of VS Code extension is blocked from current IP address. Retry #%v from 5 in 1 minute", i)
		time.Sleep(1 * time.Minute)
		err = b.ioUtil.Download(URL, dest)
	}

	if isRateLimitError(err) {
		err = errors.New("VS Code marketplace access rate limit reached. Download of VS Code extension is blocked from current IP address. 5 retries failed in 5 minutes. Giving up")
	}

	return err
}

func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	herr, ok := err.(*utils.HTTPError)
	if ok {
		return herr.StatusCode == http.StatusTooManyRequests
	}
	return false
}

func (b *brokerImpl) fetchExtensionInfo(extension string, meta model.PluginMeta) ([]byte, error) {
	re := regexp.MustCompile(`^vscode:extension/(.*)`)
	groups := re.FindStringSubmatch(extension)
	if len(groups) != 2 {
		return nil, fmt.Errorf("Parsing of VS Code extension ID '%s' failed for plugin '%s'. Extension should start from 'vscode:extension/'", extension, meta.ID)
	}
	extName := groups[1]
	body := []byte(fmt.Sprintf(bodyFmt, extName))
	req, err := http.NewRequest("POST", marketplace, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("VS Code extension id '%s' fetching failed for plugin %s. Error: %s", extension, meta.ID, err)
	}
	req.Header.Set("Accept", "application/json;api-version=3.0-preview.1")
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VS Code extension downloading failed %s. Error: %s", meta.ID, err)
	}
	defer utils.Close(resp.Body)
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("VS Code extension downloading failed %s. Error: %s", meta.ID, err)
	}
	if resp.StatusCode != 200 {
		errMsg := "VS Code extension downloading failed %s. Status: %v. Body: " + string(body)
		return nil, fmt.Errorf(errMsg, meta.ID, resp.StatusCode)
	}

	return body, nil
}

func findAssetURL(response []byte, meta model.PluginMeta) (string, error) {
	obj := &marketplaceResponse{}
	err := json.Unmarshal(response, obj)
	if err != nil {
		return "", fmt.Errorf("Failed to parse VS Code extension marketplace response for plugin %s", meta.ID)
	}
	switch {
	case len(obj.Results) == 0,
		len(obj.Results[0].Extensions) == 0,
		len(obj.Results[0].Extensions[0].Versions) == 0,
		len(obj.Results[0].Extensions[0].Versions[0].Files) == 0:

		return "", fmt.Errorf("Failed to parse VS Code extension marketplace response for plugin %s", meta.ID)
	}
	for _, f := range obj.Results[0].Extensions[0].Versions[0].Files {
		if f.AssetType == assetType {
			return f.Source, nil
		}
	}
	return "", fmt.Errorf("VS Code extension archive information is not found in marketplace response for plugin %s", meta.ID)
}
