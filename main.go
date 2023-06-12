package terraform

import (
	ahoy_targets "gitlab.com/hidothealth/platform/ahoy/src/target"
)

var KnownTargets = ahoy_targets.TargetCreatorMap{
	"terraform":        TerraformConfig{},
	"terraform_module": TerraformModuleConfig{},
}
