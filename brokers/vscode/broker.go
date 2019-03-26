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

	"github.com/eclipse/che-go-jsonrpc"
	"github.com/eclipse/che-plugin-broker/brokers/theia"
	"github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/storage"
	"github.com/eclipse/che-plugin-broker/utils"
	"github.com/eclipse/che-plugin-broker/cfg"
)

const marketplace = "https://marketplace.visualstudio.com/_apis/public/gallery/extensionquery"
const bodyFmt = `{"filters":[{"criteria":[{"filterType":7,"value":"%s"}],"pageNumber":1,"pageSize":1,"sortBy":0, "sortOrder":0 }],"assetTypes":["Microsoft.VisualStudio.Services.VSIXPackage"],"flags":131}`
const assetType = "Microsoft.VisualStudio.Services.VSIXPackage"
const errorMutuallyExclusiveExtFieldsTemplate = "VS Code extension description of the plugin '%s:%s' contains more than one mutually exclusive field 'attributes.extension', 'url', 'extensions'"
const errorNoExtFieldsTemplate = "Neither 'extension' nor 'url' nor 'extensions' field found in VS Code extension description of the plugin '%s:%s'"

// Broker is used to process VS Code extensions to run them as Che plugins
type Broker interface {
	Start(metas []model.PluginMeta)
	PushEvents(tun *jsonrpc.Tunnel)
	ProcessPlugin(meta model.PluginMeta) error
}

type brokerImpl struct {
	common.Broker
	ioUtil  utils.IoUtil
	storage *storage.Storage
	client  *http.Client
	rand    common.Random
}

// NewBroker creates Che VS Code extension broker instance
func NewBroker() *brokerImpl {
	return &brokerImpl{
		Broker:  common.NewBroker(),
		ioUtil:  utils.New(),
		storage: storage.New(),
		client:  &http.Client{},
		rand:    common.NewRand(),
	}
}

// NewBroker creates Che VS Code extension broker instance
func NewBrokerWithParams(broker common.Broker, ioUtil utils.IoUtil, storage *storage.Storage, rand common.Random, httpClient *http.Client) *brokerImpl {
	return &brokerImpl{
		Broker:  broker,
		ioUtil:  ioUtil,
		rand:    rand,
		storage: storage,
		client:  httpClient,
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

	plugins, err := b.storage.Plugins()
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
	b.PrintDebug("Stared processing plugin '%s:%s'", meta.ID, meta.Version)

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

	image := meta.Attributes["containerImage"]
	if image == "" {
		if (! cfg.OnlyApplyMetadataActions) {
			// regular plugin
			err = b.injectLocalPlugin(meta, archivesPaths)
			if err != nil {
				return err
			}
			return b.storage.AddPlugin(&meta, &model.ToolingConf{})
		}
	}
	// remote plugin
	return b.injectRemotePlugin(meta, image, archivesPaths, workDir)
}

func (b *brokerImpl) injectLocalPlugin(meta model.PluginMeta, archivesPaths []string) error {
	b.PrintDebug("Copying VS Code plugin '%s:%s'", meta.ID, meta.Version)
	for _, path := range archivesPaths {
		pluginName := b.generatePluginArchiveName(meta)
		pluginPath := filepath.Join("/plugins", pluginName)
		b.PrintDebug("Copying VS Code extension archive from '%s' to '%s' for plugin '%s:%s'", path, pluginPath, meta.ID, meta.Version)
		err := b.ioUtil.CopyFile(path, pluginPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *brokerImpl) injectRemotePlugin(meta model.PluginMeta, image string, archivesPaths []string, workDir string) error {
	tooling := theia.GenerateSidecar(image, b.rand)
	for _, archive := range archivesPaths {
		// Unzip it
		unpackedPath := filepath.Join(workDir, "extension", b.rand.String(10))
		b.PrintDebug("Unzipping archive '%s' for plugin '%s:%s' to '%s'", archive, meta.ID, meta.Version, unpackedPath)
		err := b.ioUtil.Unzip(archive, unpackedPath)
		if err != nil {
			return err
		}

		pj, err := b.getPackageJSON(unpackedPath)
		if err != nil {
			return err
		}

		if (! cfg.OnlyApplyMetadataActions) {
			pluginName := b.generatePluginFolderName(meta, *pj)

			pluginFolderPath := filepath.Join("/plugins", pluginName)
			b.PrintDebug("Copying VS Code extension '%s:%s' from '%s' to '%s'", meta.ID, meta.Version, unpackedPath, pluginFolderPath)
			err = b.ioUtil.CopyResource(unpackedPath, pluginFolderPath)
			if err != nil {
				return err
			}
		}
		theia.AddExtension(tooling, *pj)
	}

	return b.storage.AddPlugin(&meta, tooling)
}

func (b *brokerImpl) downloadArchives(URLs []string, meta model.PluginMeta, workDir string) ([]string, error) {
	paths := make([]string, 0)
	for _, URL := range URLs {
		archivePath := filepath.Join(workDir, "pluginArchive"+b.rand.String(10))
		b.PrintDebug("Downloading VS Code extension archive '%s' for plugin '%s:%s' to '%s'", URL, meta.ID, meta.Version, archivePath)
		b.PrintInfo("Downloading VS Code extension for plugin '%s:%s'", meta.ID, meta.Version)
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
	isSet := false

	if meta.URL != "" {
		isSet = true
		URLs = append(URLs, meta.URL)
	}
	if meta.Attributes != nil && meta.Attributes["extension"] != "" {
		if isSet {
			return nil, nil, fmt.Errorf(errorMutuallyExclusiveExtFieldsTemplate, meta.ID, meta.Version)
		}
		isSet = true
		extensions = append(extensions, meta.Attributes["extension"])
	}
	if meta.Extensions != nil && len(meta.Extensions) != 0 {
		if isSet {
			return nil, nil, fmt.Errorf(errorMutuallyExclusiveExtFieldsTemplate, meta.ID, meta.Version)
		}
		isSet = true
		for _, v := range meta.Extensions {
			ext, URL := extensionOrURL(v)
			switch {
			case ext != "":
				extensions = append(extensions, ext)
			case URL != "":
				URLs = append(URLs, URL)
			}
		}
	}
	if !isSet {
		return nil, nil, fmt.Errorf(errorNoExtFieldsTemplate, meta.ID, meta.Version)
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

func (b *brokerImpl) generatePluginFolderName(meta model.PluginMeta, pj model.PackageJSON) string {
	var re = regexp.MustCompile(`[^a-zA-Z_0-9]+`)
	prettyID := re.ReplaceAllString(pj.Publisher+"_"+pj.Name, "")
	return fmt.Sprintf("%s.%s.%s", meta.ID, meta.Version, prettyID)
}

func (b *brokerImpl) generatePluginArchiveName(meta model.PluginMeta) string {
	return fmt.Sprintf("%s.%s.%s.vsix", meta.ID, meta.Version, b.rand.String(10))
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
		return nil, fmt.Errorf("Parsing of VS Code extension ID '%s' failed for plugin '%s:%s'. Extension should start from 'vscode:extension/'", extension, meta.ID, meta.Version)
	}
	extName := groups[1]
	body := []byte(fmt.Sprintf(bodyFmt, extName))
	req, err := http.NewRequest("POST", marketplace, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("VS Code extension id '%s' fetching failed for plugin %s:%s. Error: %s", extension, meta.ID, meta.Version, err)
	}
	req.Header.Set("Accept", "application/json;api-version=3.0-preview.1")
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VS Code extension downloading failed %s:%s. Error: %s", meta.ID, meta.Version, err)
	}
	defer utils.Close(resp.Body)
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("VS Code extension downloading failed %s:%s. Error: %s", meta.ID, meta.Version, err)
	}
	if resp.StatusCode != 200 {
		errMsg := "VS Code extension downloading failed %s:%s. Status: %v. Body: " + string(body)
		return nil, fmt.Errorf(errMsg, meta.ID, meta.Version, resp.StatusCode)
	}

	return body, nil
}

func findAssetURL(response []byte, meta model.PluginMeta) (string, error) {
	obj := &marketplaceResponse{}
	err := json.Unmarshal(response, obj)
	if err != nil {
		return "", fmt.Errorf("Failed to parse VS Code extension marketplace response for plugin %s:%s", meta.ID, meta.Version)
	}
	switch {
	case len(obj.Results) == 0,
		len(obj.Results[0].Extensions) == 0,
		len(obj.Results[0].Extensions[0].Versions) == 0,
		len(obj.Results[0].Extensions[0].Versions[0].Files) == 0:

		return "", fmt.Errorf("Failed to parse VS Code extension marketplace response for plugin %s:%s", meta.ID, meta.Version)
	}
	for _, f := range obj.Results[0].Extensions[0].Versions[0].Files {
		if f.AssetType == assetType {
			return f.Source, nil
		}
	}
	return "", fmt.Errorf("VS Code extension archive information is not found in marketplace response for plugin %s:%s", meta.ID, meta.Version)
}

func (b *brokerImpl) getPackageJSON(pluginFolder string) (*model.PackageJSON, error) {
	packageJSONPath := filepath.Join(pluginFolder, "extension", "package.json")
	f, err := ioutil.ReadFile(packageJSONPath)
	if err != nil {
		return nil, err
	}
	pj := &model.PackageJSON{}
	err = json.Unmarshal(f, pj)
	return pj, err
}
