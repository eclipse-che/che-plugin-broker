package utils

import (
	"fmt"
	"strings"

	"github.com/eclipse/che-plugin-broker/model"
)

// ValidateMetas ensures that a plugin meta conforms to expectations at a basic level, e.g. that
// required fields are present.
func ValidateMetas(metas ...model.PluginMeta) error {
	for _, meta := range metas {
		switch meta.APIVersion {
		case "":
			return fmt.Errorf("Plugin '%s' is invalid. Field 'apiVersion' must be present", meta.ID)
		case "v2":
			// validate here something
		default:
			return fmt.Errorf("Plugin '%s' is invalid. Field 'apiVersion' contains invalid version '%s'", meta.ID, meta.APIVersion)
		}

		switch strings.ToLower(meta.Type) {
		case ChePluginType:
			fallthrough
		case EditorPluginType:
			if len(meta.Spec.Extensions) != 0 {
				return fmt.Errorf("Plugin '%s' is invalid. Field 'spec.extensions' is not allowed in plugin of type '%s'", meta.ID, meta.Type)
			}
			if len(meta.Spec.Containers) == 0 {
				return fmt.Errorf("Plugin '%s' is invalid. Field 'spec.containers' must not be empty", meta.ID)
			}
		case TheiaPluginType:
			fallthrough
		case VscodePluginType:
			if len(meta.Spec.Extensions) == 0 {
				return fmt.Errorf("Plugin '%s' is invalid. Field 'spec.extensions' must not be empty", meta.ID)
			}
			if len(meta.Spec.Containers) > 1 {
				return fmt.Errorf("Plugin '%s' is invalid. Containers list 'spec.containers' must not contain more than 1 container, but '%d' found", meta.ID, len(meta.Spec.Containers))
			}
			if len(meta.Spec.Endpoints) != 0 {
				return fmt.Errorf("Plugin '%s' is invalid. Setting endpoints at 'spec.endpoints' is not allowed in plugins of type '%s'", meta.ID, meta.Type)
			}
		case "":
			return fmt.Errorf("Type field is missing in meta information of plugin '%s'", meta.ID)
		default:
			return fmt.Errorf("Type '%s' of plugin '%s' is unsupported", meta.Type, meta.ID)
		}
	}
	return nil
}
