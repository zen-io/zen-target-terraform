package terraform

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	ahoy_targets "gitlab.com/hidothealth/platform/ahoy/src/target"
	"golang.org/x/exp/slices"
)

type TerraformDeploymentConfig struct {
	VarFiles                  []string `mapstructure:"var_files" desc:"Variable files to include (.tfvars)"`
	Backend                   *string  `mapstructure:"backend" desc:"Terraform backend file. Can be a ref or path"`
	Terraform                 *string  `mapstructure:"terraform" desc:"Terraform executable. Can be a ref or path"`
	Tflocal                   *string  `mapstructure:"tflocal" desc:"Tflocal executable. Can be a ref or path"`
	Tflint                    *string  `mapstructure:"tflint" desc:"Tflint executable. Can be a ref or path"`
	Modules                   []string `mapstructure:"modules" desc:"Modules to include as sources. List of references"`
	ProviderConfigs           []string `mapstructure:"provider_configs" desc:"Providers to include as sources"`
	AllowFailure              bool     `mapstructure:"allow_failure"`
	ahoy_targets.DeployFields `mapstructure:",squash"`
}

type TerraformConfig struct {
	Srcs                      []string `mapstructure:"srcs" desc:"Terraform source files (.tf)"`
	Data                      []string `mapstructure:"data" desc:"Other files to add to this execution, that wont be interpolated"`
	DeployDeps                []string `mapstructure:"deploy_deps" desc:"Deploy dependencies"`
	TerraformDeploymentConfig `mapstructure:",squash"`
	ahoy_targets.BaseFields   `mapstructure:",squash"`
}

func (tc TerraformConfig) GetTargets(tcc *ahoy_targets.TargetConfigContext) ([]*ahoy_targets.Target, error) {
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

		if ahoy_targets.IsTargetReference(pc) {
			tc.Deps = append(tc.Deps, pc)
		}
	}

	for _, mod := range tc.Modules {
		buildSrcs["modules"] = append(buildSrcs["modules"], mod)
		if ahoy_targets.IsTargetReference(mod) {
			tc.Deps = append(tc.Deps, mod)
		}
	}

	var outs []string
	if tc.Environments != nil && len(tc.Environments) > 0 {
		var backend string
		for env, envConf := range tc.Environments {
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
				if ahoy_targets.IsTargetReference(backend) {
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
			if ahoy_targets.IsTargetReference(val) {
				tc.Deps = append(tc.Deps, val)
			}
		}
	}

	steps := []*ahoy_targets.Target{
		ahoy_targets.NewTarget(
			tc.Name,
			ahoy_targets.WithSrcs(buildSrcs),
			ahoy_targets.WithOuts(outs),
			ahoy_targets.WithEnvironments(tc.Environments),
			ahoy_targets.WithTools(tc.Tools),
			ahoy_targets.WithEnvVars(tc.Env),
			ahoy_targets.WithPassEnv(tc.PassEnv),
			ahoy_targets.WithTargetScript("build", &ahoy_targets.TargetScript{
				Deps: tc.Deps,
				Run: func(target *ahoy_targets.Target, runCtx *ahoy_targets.RuntimeContext) error {
					flattenedSrcs := target.Srcs["_srcs"]
					flattenedSrcs = append(flattenedSrcs, target.Srcs["providers"]...)

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
							envInterpolate["ENV"] = env
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

						for _, src := range flattenedSrcs {
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
								to = filepath.Join(dest, filepath.Base(src))
							}

							if err := ahoy_targets.CopyWithInterpolate(from, to, target, runCtx, envInterpolate); err != nil {
								return fmt.Errorf("copying flattened src: %w", err)
							}
						}

						for _, src := range target.Srcs[backendPath] {
							from := src
							to := filepath.Join(dest, fmt.Sprintf("_backend_%s", filepath.Base(src)))

							if err := ahoy_targets.CopyWithInterpolate(from, to, target, runCtx, envInterpolate); err != nil {
								return fmt.Errorf("copying src: %w", err)
							}
						}

						for _, src := range target.Srcs["modules"] {
							from := src
							to := filepath.Join(dest, src)

							if err := ahoy_targets.CopyWithInterpolate(from, to, target, runCtx, envInterpolate); err != nil {
								return fmt.Errorf("copying module %w", err)
							}
						}
					}

					return nil
				},
			}),
			ahoy_targets.WithTargetScript("deploy", &ahoy_targets.TargetScript{
				Deps:  tc.DeployDeps,
				Alias: []string{"apply"},
				Pre:   preFunc,
				Run: func(target *ahoy_targets.Target, runCtx *ahoy_targets.RuntimeContext) error {
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
			}),
			ahoy_targets.WithTargetScript("lint", &ahoy_targets.TargetScript{
				Run: func(target *ahoy_targets.Target, runCtx *ahoy_targets.RuntimeContext) error {
					if _, ok := target.Tools["tflint"]; !ok {
						return fmt.Errorf("tflint is not configured")
					}

					target.SetStatus(fmt.Sprintf("Linting %s", target.Qn()))
					target.Debugln("executing %s", strings.Join([]string{target.Tools["tflint"]}, " "))
					cmd := exec.Command(target.Tools["tflint"])
					cmd.Dir = target.Cwd
					cmd.Env = target.GetEnvironmentVariablesList()
					cmd.Stdout = target
					cmd.Stderr = target
					return cmd.Run()
				},
			}),
			ahoy_targets.WithTargetScript("remove", &ahoy_targets.TargetScript{
				Pre: preFunc,
				Run: func(target *ahoy_targets.Target, runCtx *ahoy_targets.RuntimeContext) error {
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
			}),
			ahoy_targets.WithTargetScript("unlock", &ahoy_targets.TargetScript{
				Pre: preFunc,
				Run: func(target *ahoy_targets.Target, runCtx *ahoy_targets.RuntimeContext) error {
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
			}),
		),
	}

	return steps, nil
}
