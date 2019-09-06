package artifacts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	jsonrpc "github.com/eclipse/che-go-jsonrpc"
	"github.com/eclipse/che-plugin-broker/common"
	"github.com/eclipse/che-plugin-broker/model"
	"github.com/eclipse/che-plugin-broker/utils"
)

const errorNoExtFieldsTemplate = "Field 'extensions' is not found in the description of the plugin '%s'"

var re = regexp.MustCompile(`[^a-zA-Z_0-9]+`)

// Broker is used to process Che plugins
type Broker struct {
	common.Broker
	ioUtils utils.IoUtil
	rand    common.Random
}

// NewBrokerWithParams creates Che broker instance with parameters
func NewBrokerWithParams(
	commonBroker common.Broker,
	ioUtil utils.IoUtil,
	rand common.Random) *Broker {
	return &Broker{
		Broker:  commonBroker,
		ioUtils: ioUtil,
		rand:    rand,
	}
}

// NewBroker creates Che broker instance
func NewBroker(localhostSidecar bool) *Broker {
	return NewBrokerWithParams(common.NewBroker(), utils.New(), common.NewRand())
}

func (b *Broker) fail(err error) error {
	b.PubFailed(err.Error())
	b.PubLog(err.Error())
	return err
}

// PushEvents sets given tunnel as consumer of broker events.
func (b *Broker) PushEvents(tun *jsonrpc.Tunnel) {
	b.Broker.PushEvents(tun, model.BrokerStatusEventType, model.BrokerResultEventType, model.BrokerLogEventType)
}

// Start downloads metas from plugin registry for specified
// pluginFQNs and then executes plugins metas processing and sending data to Che master
func (b *Broker) Start(pluginFQNs []model.PluginFQN, defaultRegistry string) error {
	b.cleanupPluginsDirectory(b.ioUtils)

	pluginMetas, err := utils.GetPluginMetas(pluginFQNs, defaultRegistry, b.ioUtils)
	if err != nil {
		return b.fail(fmt.Errorf("Failed to download plugin meta: %s", err))
	}
	defer b.CloseConsumers()
	b.PubStarted()
	b.PrintInfo("Provision plugin broker")
	b.PrintPlan(pluginMetas)

	err = utils.ResolveRelativeExtensionPaths(pluginMetas, defaultRegistry)
	if err != nil {
		return b.fail(err)
	}

	for _, meta := range pluginMetas {
		err = b.ProcessPlugin(meta)
		if err != nil {
			return b.fail(err)
		}
	}

	// TODO
	b.PrintInfo("All plugins have been successfully downloaded")
	// b.PrintDebug(result)
	b.PubDone("TODO")

	return nil
}

func (b *Broker) cleanupPluginsDirectory(ioUtil utils.IoUtil) {
	b.PrintInfo("Cleaning /plugins dir")
	files, err := filepath.Glob(filepath.Join("/plugins", "*"))
	if err != nil {
		// Send log about clearing failure but proceed.
		// We might want to change this behavior later
		b.PrintInfo("WARN: failed to clear /plugins directory. Error: %s", err)
		return
	}

	for _, file := range files {
		err = os.RemoveAll(file)
		if err != nil {
			b.PrintInfo("WARN: failed to remove '%s'. Error: %s", file, err)
		}
	}
}

// ProcessPlugin processes metas of different plugin types and passes metas of each particular type
// to the appropriate plugin broker
func (b *Broker) ProcessPlugin(meta model.PluginMeta) error {
	if !utils.IsTheiaOrVscodePlugin(meta) {
		return nil
	}

	URLs, err := getUrls(meta)
	if err != nil {
		return err
	}

	workDir, err := b.ioUtils.TempDir("", "vscode-extension-broker")
	if err != nil {
		return err
	}

	archivesPaths, err := b.downloadArchives(URLs, meta, workDir)
	if err != nil {
		return err
	}

	err = b.injectPlugin(meta, archivesPaths)
	return err
}

func (b *Broker) downloadArchives(URLs []string, meta model.PluginMeta, workDir string) ([]string, error) {
	paths := make([]string, 0)
	for _, URL := range URLs {
		archivePath := b.ioUtils.ResolveDestPathFromURL(URL, workDir)
		b.PrintDebug("Downloading VS Code extension archive '%s' for plugin '%s' to '%s'", URL, meta.ID, archivePath)
		b.PrintInfo("Downloading VS Code extension for plugin '%s'", meta.ID)
		archivePath, err := b.ioUtils.Download(URL, archivePath, true)
		paths = append(paths, archivePath)
		if err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func (b *Broker) injectPlugin(meta model.PluginMeta, archivesPaths []string) error {
	for _, path := range archivesPaths {
		pluginPath := "/plugins"
		if len(meta.Spec.Containers) > 0 {
			// Plugin is remote
			pluginUniqueName := re.ReplaceAllString(meta.Publisher+"_"+meta.Name+"_"+meta.Version, `_`)
			pluginPath = filepath.Join(pluginPath, "sidecars", pluginUniqueName)
			err := b.ioUtils.MkDir(pluginPath)
			if err != nil {
				return err
			}
		}
		pluginArchiveName := b.generatePluginArchiveName(meta, path)
		pluginArchivePath := filepath.Join(pluginPath, pluginArchiveName)
		err := b.ioUtils.CopyFile(path, pluginArchivePath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Broker) generatePluginArchiveName(plugin model.PluginMeta, archivePath string) string {
	archiveName := filepath.Base(archivePath)
	return fmt.Sprintf("%s.%s.%s.%s.%s", plugin.Publisher, plugin.Name, plugin.Version, b.rand.String(10), archiveName)
}

func getUrls(meta model.PluginMeta) ([]string, error) {
	URLs := make([]string, 0)
	if len(meta.Spec.Extensions) == 0 {
		return nil, fmt.Errorf(errorNoExtFieldsTemplate, meta.ID)
	}
	for _, URL := range meta.Spec.Extensions {
		URLs = append(URLs, URL)
	}
	return URLs, nil
}
