package terraform

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	environs "github.com/zen-io/zen-core/environments"
	zen_targets "github.com/zen-io/zen-core/target"
	"github.com/zen-io/zen-core/utils"
	"golang.org/x/exp/slices"
)

type TerraformDeploymentConfig struct {
	VarFiles        []string `mapstructure:"var_files" desc:"Variable files to include (.tfvars)"`
	Backend         *string  `mapstructure:"backend" desc:"Terraform backend file. Can be a ref or path"`
	Terraform       *string  `mapstructure:"terraform" desc:"Terraform executable. Can be a ref or path"`
	Tflocal         *string  `mapstructure:"tflocal" desc:"Tflocal executable. Can be a ref or path"`
	Tflint          *string  `mapstructure:"tflint" desc:"Tflint executable. Can be a ref or path"`
	Modules         []string `mapstructure:"modules" desc:"Modules to include as sources. Can have references"`
	ProviderConfigs []string `mapstructure:"provider_configs" desc:"Providers to include as sources"`
	AllowFailure    bool     `mapstructure:"allow_failure"`
}

type DeployConfig struct {
	Deps      []string          `mapstructure:"deps" zen:"yes" desc:"Build dependencies"`
	PassEnv   []string          `mapstructure:"pass_env" zen:"yes" desc:"List of environment variable names that will be passed from the OS environment, they are part of the target hash"`
	SecretEnv []string          `mapstructure:"secret_env" zen:"yes" desc:"List of environment variable names that will be passed from the OS environment, they are not used to calculate the target hash"`
	Env       map[string]string `mapstructure:"env" zen:"yes" desc:"Key-Value map of static environment variables to be used"`
	Outs      []string          `mapstructure:"outs" zen:"yes"`
}

type TerraformConfig struct {
	Name                      string                           `mapstructure:"name" zen:"yes" desc:"Name for the target"`
	Description               string                           `mapstructure:"desc" zen:"yes" desc:"Target description"`
	Labels                    []string                         `mapstructure:"labels" zen:"yes" desc:"Labels to apply to the targets"`
	Deps                      []string                         `mapstructure:"deps" zen:"yes" desc:"Build dependencies"`
	PassEnv                   []string                         `mapstructure:"pass_env" zen:"yes" desc:"List of environment variable names that will be passed from the OS environment, they are part of the target hash"`
	PassSecretEnv             []string                         `mapstructure:"pass_secret_env" zen:"yes" desc:"List of environment variable names that will be passed from the OS environment, they are not used to calculate the target hash"`
	Env                       map[string]string                `mapstructure:"env" zen:"yes" desc:"Key-Value map of static environment variables to be used"`
	Tools                     map[string]string                `mapstructure:"tools" zen:"yes" desc:"Key-Value map of tools to include when executing this target. Values can be references"`
	Visibility                []string                         `mapstructure:"visibility" zen:"yes" desc:"List of visibility for this target"`
	Environments              map[string]*environs.Environment `mapstructure:"environments" zen:"yes" desc:"Deployment Environments"`
	Deploy                    *DeployConfig                    `mapstructure:"deploy"`
	Srcs                      []string                         `mapstructure:"srcs" desc:"Terraform source files (.tf)"`
	Data                      []string                         `mapstructure:"data" desc:"Other files to add to this execution, that wont be interpolated"`
	TerraformDeploymentConfig `mapstructure:",squash"`
}

func (tc TerraformConfig) GetTargets(tcc *zen_targets.TargetConfigContext) ([]*zen_targets.TargetBuilder, error) {
	buildSrcs := map[string][]string{
		"_srcs":     tc.Srcs,
		"_data":     tc.Data,
		"providers": {},
		"modules":   {},
	}

	if len(tc.Tools) == 0 {
		tc.Tools = map[string]string{}
	}
	var err error
	tc.Tools["terraform"], err = tcc.ResolveToolchain(tc.Terraform, "terraform", tc.Tools)
	if err != nil {
		return nil, err
	}
	tc.Tools["tflocal"], err = tcc.ResolveToolchain(tc.Tflocal, "tflocal", tc.Tools)
	if err != nil {
		return nil, err
	}
	tc.Tools["tflint"], err = tcc.ResolveToolchain(tc.Tflint, "tflint", tc.Tools)
	if err != nil {
		return nil, err
	}

	for _, pc := range tc.ProviderConfigs {
		buildSrcs["providers"] = append(buildSrcs["providers"], pc)

		if zen_targets.IsTargetReference(pc) {
			tc.Deps = append(tc.Deps, pc)
		}
	}

	for _, mod := range tc.Modules {
		if zen_targets.IsTargetReference(mod) {
			tc.Deps = append(tc.Deps, mod)
		} else {
			buildSrcs["modules"] = append(buildSrcs["modules"], fmt.Sprintf("%s/**", mod))
		}

		tc.Labels = append(tc.Labels, fmt.Sprintf("module=%s=%s", mod, filepath.Base(mod)))
	}

	var outs []string
	if tc.Environments != nil && len(tc.Environments) > 0 {
		for env, envConf := range tc.Environments {
			var backend string
			if tc.Backend != nil {
				backend = *tc.Backend
			} else if val, ok := envConf.Variables["TERRAFORM_BACKEND"]; ok {
				backend = val
			} else if e, ok := tcc.Environments[env]; ok && e.Variables != nil {
				if val, ok := e.Variables["TERRAFORM_BACKEND"]; ok {
					backend = val
				}
			}
			if backend != "" {
				if zen_targets.IsTargetReference(backend) {
					tc.Deps = append(tc.Deps, backend)
				}
				buildSrcs["backend_"+env] = []string{backend}
			}
			outs = append(outs, fmt.Sprintf("%s/**/*", env))
		}
	} else {
		outs = []string{"**"}

		if tc.Backend != nil {
			buildSrcs["backend"] = []string{*tc.Backend}
		} else if val, ok := tcc.Variables["TERRAFORM_BACKEND"]; ok {
			buildSrcs["backend"] = []string{val}
			if zen_targets.IsTargetReference(val) {
				tc.Deps = append(tc.Deps, val)
			}
		}
	}

	t := zen_targets.ToTarget(tc)
	t.Srcs = buildSrcs
	t.Outs = outs
	t.NoCacheInterpolation = false

	t.Scripts = map[string]*zen_targets.TargetBuilderScript{
		"build": {
			Run: func(target *zen_targets.Target, runCtx *zen_targets.RuntimeContext) error {
				envs := []string{}
				if tc.Environments != nil && len(tc.Environments) > 0 {
					for env := range tc.Environments {
						envs = append(envs, env)
					}
				} else {
					envs = append(envs, "")
				}

				for _, env := range envs {
					var dest, backendPath string
					envInterpolate := make(map[string]string)
					if env != "" {
						dest = filepath.Join(target.Cwd, env)
						envInterpolate["DEPLOY_ENV"] = env
						backendPath = "backend_" + env
					} else {
						dest = target.Cwd
						backendPath = "backend"
					}

					varFilesFilter := []string{}
					for _, v := range tc.VarFiles {
						interpolatedVarName, err := target.Interpolate(v, envInterpolate)
						if err != nil {
							return fmt.Errorf("interpolating var file name: %w", err)
						}
						varFilesFilter = append(varFilesFilter, interpolatedVarName)
					}

					for _, src := range target.Srcs["_srcs"] {
						var from, to string

						if strings.HasSuffix(src, ".tfvars") || strings.HasSuffix(src, ".tfvars.json") {
							i := slices.IndexFunc(varFilesFilter, func(item string) bool {
								return strings.HasSuffix(src, item)
							})
							if i == -1 {
								continue
							}

							name := filepath.Base(varFilesFilter[i])
							from = src
							if strings.HasSuffix(name, ".tfvars") {
								to = filepath.Join(dest, fmt.Sprintf("%d-%s.auto.tfvars", i, strings.TrimSuffix(name, ".tfvars")))
							} else if strings.HasSuffix(name, ".tfvars.json") {
								to = filepath.Join(dest, fmt.Sprintf("%d-%s.auto.tfvars.json", i, strings.TrimSuffix(name, ".tfvars.json")))
							}
						} else {
							from = src
							to = filepath.Join(dest, filepath.Base(target.StripCwd(src)))
						}

						if err := utils.Copy(from, to); err != nil {
							return fmt.Errorf("copying flattened src: %w", err)
						}
					}

					for _, src := range target.Srcs["providers"] {
						from := src
						to := filepath.Join(dest, filepath.Base(target.StripCwd(src)))

						if err := target.Copy(from, to, envInterpolate); err != nil {
							return fmt.Errorf("copying provider: %w", err)
						}
					}

					for _, src := range target.Srcs[backendPath] {
						from := src
						to := filepath.Join(dest, fmt.Sprintf("_backend_%s", filepath.Base(target.StripCwd(src))))

						if err := target.Copy(from, to, envInterpolate); err != nil {
							return fmt.Errorf("copying backend: %w", err)
						}
					}

					for _, label := range target.Labels {
						if strings.HasPrefix(label, "module=") {
							info := strings.Split(strings.TrimPrefix(label, "module="), "=")
							from := filepath.Join(target.Cwd, info[0])
							to := filepath.Join(dest, info[1])
		
							if err := utils.Link(from, to); err != nil { // we do not want to interpolate here
								return fmt.Errorf("copying module %w", err)
							}
						}
					}
				}

				return nil
			},
		},
		"deploy": {
			Alias: []string{"apply"},
			Pre:   preFunc,
			TransformOut: func(target *zen_targets.Target, o string) (string, bool) {
				return filepath.Base(o), true
			},
			Run: func(target *zen_targets.Target, runCtx *zen_targets.RuntimeContext) error {
				target.SetStatus(fmt.Sprintf("Initializing %s", target.Qn()))
				if err := tfInit(target, runCtx.Env); err != nil {
					return fmt.Errorf("deploying: %s", err)
				}

				if runCtx.DryRun {
					target.SetStatus(fmt.Sprintf("Planning %s", target.Qn()))
					if err := tfPlanApply(target, runCtx.Env); err != nil {
						return fmt.Errorf("deploying: %s", err)
					}
				} else {
					target.SetStatus(fmt.Sprintf("Applying %s", target.Qn()))
					if err := tfApply(target, runCtx.Env); err != nil && !tc.AllowFailure {
						return fmt.Errorf("deploying: %s", err)
					}
				}

				return nil
			},
		},
		"lint": {
			Run: func(target *zen_targets.Target, runCtx *zen_targets.RuntimeContext) error {
				if _, ok := target.Tools["tflint"]; !ok {
					return fmt.Errorf("tflint is not configured")
				}

				target.SetStatus(fmt.Sprintf("Linting %s", target.Qn()))

				return target.Exec([]string{target.Tools["tflint"]}, "tf lint")
			},
		},
		"remove": {
			Alias: []string{"rm", "del", "delete"},
			Pre:   preFunc,
			Run: func(target *zen_targets.Target, runCtx *zen_targets.RuntimeContext) error {
				target.SetStatus(fmt.Sprintf("Initializing %s", target.Qn()))
				if err := tfInit(target, runCtx.Env); err != nil {
					return fmt.Errorf("destroying: %s", err)
				}

				if runCtx.DryRun {
					target.SetStatus(fmt.Sprintf("Planning %s", target.Qn()))
					if err := tfPlanDestroy(target, runCtx.Env); err != nil {
						return fmt.Errorf("destroying: %s", err)
					}
				} else {
					target.SetStatus(fmt.Sprintf("Applying %s", target.Qn()))
					if err := tfDestroy(target, runCtx.Env); err != nil {
						return fmt.Errorf("destroying: %s", err)
					}
				}

				return nil
			},
		},
		"unlock": {
			Pre: preFunc,
			Run: func(target *zen_targets.Target, runCtx *zen_targets.RuntimeContext) error {
				target.SetStatus(fmt.Sprintf("Initializing %s", target.Qn()))
				if err := tfInit(target, runCtx.Env); err != nil {
					return fmt.Errorf("destroying: %s", err)
				}

				target.SetStatus(fmt.Sprintf("Planning should return lock info for %s", target.Qn()))
				var executable string
				if runCtx.Env == "local" {
					executable = "tflocal"
				} else {
					executable = "terraform"
				}

				cmd := exec.Command(target.Tools[executable], "plan")
				cmd.Dir = target.Cwd
				cmd.Env = target.GetEnvironmentVariablesList()
				out, err := cmd.Output()
				if err == nil {
					return fmt.Errorf("nothing to unlock, plan succeeded")
				}

				id := string(regexp.MustCompile(`ID:\s+([^\n]+)`).FindSubmatch(out)[1])

				return terraformExec(target, runCtx.Env, []string{"force-unlock", "-force", id})
			},
		},
	}

	if tc.Deploy != nil {
		for scriptName, script := range t.Scripts {
			if scriptName == "build" {
				continue
			} else if scriptName == "deploy" {
				script.Deps = tc.Deploy.Deps
			}

			script.Env = tc.Deploy.Env
			script.PassEnv = tc.Deploy.PassEnv
			script.PassSecretEnv = tc.Deploy.SecretEnv
		}

		t.Scripts["deploy"].Outs = tc.Deploy.Outs
	}

	return []*zen_targets.TargetBuilder{t}, nil
}
