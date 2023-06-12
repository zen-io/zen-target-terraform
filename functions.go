package terraform

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	ahoy_targets "gitlab.com/hidothealth/platform/ahoy/src/target"
)

var terraformExec = func(target *ahoy_targets.Target, env string, args []string) error {
	var executable string
	if env == "local" {
		executable = "tflocal"
	} else {
		executable = "terraform"
	}

	target.Debugln("executing %s", strings.Join(append([]string{target.Tools[executable]}, args...), " "))
	cmd := exec.Command(target.Tools[executable], args...)
	cmd.Dir = target.Cwd
	cmd.Env = append(target.GetEnvironmentVariablesList(), "TF_INPUT=false")
	cmd.Stdout = target
	cmd.Stderr = target
	return cmd.Run()
}

var tfInit = func(target *ahoy_targets.Target, env string) error {
	if err := terraformExec(target, env, []string{"init"}); err != nil {
		return fmt.Errorf("executing init: %w", err)
	}

	return nil
}

var tfPlanApply = func(target *ahoy_targets.Target, env string) error {
	if err := terraformExec(target, env, []string{"plan"}); err != nil {
		return fmt.Errorf("executing plan: %w", err)
	}

	return nil
}

var tfPlanDestroy = func(target *ahoy_targets.Target, env string) error {
	if err := terraformExec(target, env, []string{"plan", "-destroy"}); err != nil {
		return fmt.Errorf("executing plan: %w", err)
	}

	return nil
}

var tfApply = func(target *ahoy_targets.Target, env string) error {
	if err := terraformExec(target, env, []string{"apply", "-auto-approve"}); err != nil {
		return fmt.Errorf("executing apply: %w", err)
	}

	return nil
}

var tfDestroy = func(target *ahoy_targets.Target, env string) error {
	if err := terraformExec(target, env, []string{"apply", "-destroy", "-auto-approve"}); err != nil {
		return fmt.Errorf("executing destroy: %w", err)
	}

	return nil
}

var preFunc = func(target *ahoy_targets.Target, runCtx *ahoy_targets.RuntimeContext) error {
	if target.Environments != nil && len(target.Environments) > 0 {
		target.Cwd = filepath.Join(target.Cwd, runCtx.Env)
	}
	return nil
}