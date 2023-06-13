package terraform

// type TerraformDataConfig struct {
// 	Name            string                       `mapstructure:"name" desc:"Rule name"`
// 	Srcs            []string                     `mapstructure:"srcs"`
// 	Outs            []string                     `mapstructure:"outs"`
// 	Labels          []string                     `mapstructure:"labels"`
// 	Deps            []string                     `mapstructure:"deps"`
// 	Hashes          []string                     `mapstructure:"hashes"`
// 	Headers         map[string]string            `mapstructure:"headers"`
// 	PassEnv         []string                     `mapstructure:"pass_env"`
// 	VarFiles        []string                     `mapstructure:"var_files"`        // Variable files to include (.tfvars)
// 	Backend         string                       `mapstructure:"backend"`          // Terraform backend file. Can be a ref or path
// 	Terraform       *string                      `mapstructure:"terraform"`        // Terraform executable. Can be a ref or path
// 	Tflocal         *string                      `mapstructure:"tflocal"`          // Tflocal executable. Can be a ref or path
// 	Modules         []string                     `mapstructure:"modules"`          // Modules to include as sources. List of references
// 	ProviderConfigs []string                     `mapstructure:"provider_configs"` // Providers to include as sources
// 	Environments    map[string]*environs.Environment `mapstructure:"environments"`     // Deployment Environments
// }

// func GetTerraformDataTargets(block interface{}, tcc *zen_targets.TargetConfigContext) ([]*zen_targets.Target, error) {
// 	var tdc TerraformDataConfig
// 	mapstructure.Decode(block, &tdc)

// 	var steps []*zen_targets.Target
// 	out := regexp.MustCompile(`([^\.]+)(?:\..*)?`).ReplaceAllString(filepath.Base(*tdc.Url), "$1")
// 	steps = append(steps, zen_targets.NewTarget(
// 		tdc.Name,
// 		zen_targets.WithHashes(tdc.Hashes),
// 		zen_targets.WithLabels(tdc.Labels),
// 		zen_targets.WithOuts([]string{out}),
// 		// 		zen_targets.WithBuildFunc(func(target zen_targets.Target, runCtx *zen_targets.RuntimeContext) error {

// 			return nil
// 		}),
// 	))

// 	return steps, nil
// }
