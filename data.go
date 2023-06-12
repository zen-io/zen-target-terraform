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

// func GetTerraformDataTargets(block interface{}, tcc *ahoy_targets.TargetConfigContext) ([]*ahoy_targets.Target, error) {
// 	var tdc TerraformDataConfig
// 	mapstructure.Decode(block, &tdc)

// 	var steps []*ahoy_targets.Target
// 	out := regexp.MustCompile(`([^\.]+)(?:\..*)?`).ReplaceAllString(filepath.Base(*tdc.Url), "$1")
// 	steps = append(steps, ahoy_targets.NewTarget(
// 		tdc.Name,
// 		ahoy_targets.WithHashes(tdc.Hashes),
// 		ahoy_targets.WithLabels(tdc.Labels),
// 		ahoy_targets.WithOuts([]string{out}),
// 		// 		ahoy_targets.WithBuildFunc(func(target ahoy_targets.Target, runCtx *ahoy_targets.RuntimeContext) error {

// 			return nil
// 		}),
// 	))

// 	return steps, nil
// }
