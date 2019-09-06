package utils

import (
	"strings"

	"github.com/eclipse/che-plugin-broker/model"
)

const ChePluginType = "che plugin"
const EditorPluginType = "che editor"
const TheiaPluginType = "theia plugin"
const VscodePluginType = "vs code extension"

func IsTheiaOrVscodePlugin(meta model.PluginMeta) bool {
	pluginType := strings.ToLower(meta.Type)
	return pluginType == TheiaPluginType || pluginType == VscodePluginType
}
