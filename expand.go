package terraform

import (
	"fmt"

	zen_targets "github.com/zen-io/zen-core/target"
)

func expandTools(tf *string, tflocal *string, tcc *zen_targets.TargetConfigContext) (tools map[string]string, deps []string, err error) {
	tools = map[string]string{}
	deps = make([]string, 0)

	if tf != nil {
		tools["terraform"] = *tf
	} else {
		if val, ok := tcc.KnownToolchains["terraform"]; !ok {
			err = fmt.Errorf("terraform toolchain is not configured")
			return
		} else {
			tools["terraform"] = val
			if zen_targets.IsTargetReference(val) {
				deps = append(deps, val)
			}
		}
	}

	if tflocal != nil {
		tools["tflocal"] = *tflocal
	} else {
		if val, ok := tcc.KnownToolchains["tflocal"]; !ok {
			err = fmt.Errorf("tflocal toolchain is not configured")
			return
		} else {
			tools["tflocal"] = val
			if zen_targets.IsTargetReference(val) {
				deps = append(deps, val)
			}
		}
	}

	return
}

func expandVarFiles(tc TerraformConfig, tcc *zen_targets.TargetConfigContext) (map[string][]string, error) {
	varsByEnv := make(map[string][]string)

	for env := range tc.Environments {
		interpolatedVars := []string{}

		if tc.VarFiles != nil {
			for _, vf := range tc.VarFiles {
				interpolated, err := tcc.Interpolate(vf, map[string]string{"ENV": env})
				if err != nil {
					return nil, err
				}
				interpolatedVars = append(interpolatedVars, interpolated)
			}

			varsByEnv["vars_"+env] = interpolatedVars
		}
	}

	return varsByEnv, nil
}
