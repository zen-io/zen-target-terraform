package terraform

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	git "github.com/go-git/go-git/v5"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	getr "github.com/hashicorp/go-getter/v2"
	files "github.com/tiagoposse/ahoy-files"
	ahoy_targets "gitlab.com/hidothealth/platform/ahoy/src/target"
)

type TerraformModuleConfig struct {
	Srcs                    []string          `mapstructure:"srcs"`
	Url                     *string           `mapstructure:"url"`
	Hashes                  []string          `mapstructure:"hashes"`
	Username                *string           `mapstructure:"username"`
	Password                *string           `mapstructure:"password"`
	Headers                 map[string]string `mapstructure:"headers,remain"`
	GitlabProject           string            `mapstructure:"gitlab_project"`
	ModuleName              string            `mapstructure:"module_name"`
	ahoy_targets.BaseFields `mapstructure:",squash"`
}

type Release struct {
	Version string `json:"version"`
	Id      string `json:"id"`
}

func (tmc TerraformModuleConfig) GetTargets(tcc *ahoy_targets.TargetConfigContext) ([]*ahoy_targets.Target, error) {
	var steps []*ahoy_targets.Target
	if tmc.Url != nil {
		out := regexp.MustCompile(`([^\.]+)(?:\..*)?`).ReplaceAllString(filepath.Base(*tmc.Url), "$1")
		steps = append(steps, ahoy_targets.NewTarget(
			tmc.Name,
			ahoy_targets.WithHashes(tmc.Hashes),
			ahoy_targets.WithLabels(tmc.Labels),
			ahoy_targets.WithOuts([]string{out}),

			ahoy_targets.WithTargetScript("build", &ahoy_targets.TargetScript{
				Deps: tmc.Deps,
				Run: func(target *ahoy_targets.Target, runCtx *ahoy_targets.RuntimeContext) error {
					if strings.HasPrefix(*tmc.Url, "git") || strings.HasSuffix(*tmc.Url, ".git") {
						// Set up authentication
						opts := &git.CloneOptions{
							URL:      *tmc.Url,
							Progress: target,
						}

						if tmc.Username != nil && tmc.Password != nil {
							opts.Auth = &gitHttp.BasicAuth{
								Username: *tmc.Username,
								Password: *tmc.Password,
							}
						}

						// Clone the repository to a local directory
						if _, err := git.PlainClone(filepath.Join(target.Cwd, out), false, opts); err != nil {
							return err
						}
					} else if _, err := getr.GetAny(context.TODO(), filepath.Join(target.Cwd, out), *tmc.Url); err != nil {
						return err
					}

					return nil
				},
			}),
		))
	} else {
		fc := files.FilegroupConfig{
			BuildFields: ahoy_targets.BuildFields{
				Name: tmc.Name,
				BaseFields: ahoy_targets.BaseFields{
					Labels: tmc.Labels,
					Deps:   tmc.Deps,
				},
				Srcs: tmc.Srcs,
			},
			Flatten: true,
		}
		if fgTargets, err := fc.GetTargets(tcc); err != nil {
			return nil, err
		} else {
			steps = fgTargets
		}
	}

	var getLatestVersion = func(ctx *ahoy_targets.Target, runCtx *ahoy_targets.RuntimeContext) (*Release, error) {
		// GitLab personal access token with "api" scope
		token := "YOUR_GITLAB_API_TOKEN"

		// GitLab project ID where the module will be uploaded
		projectID := tmc.GitlabProject

		// GitLab API endpoint for uploading a file
		apiEndpoint := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/packages?package_type=terraform_module&package_name=%s&order_by=version&sort=desc", projectID, tmc.ModuleName)

		// Create a new HTTP request with the multipart form as the body
		request, err := http.NewRequest(http.MethodDelete, apiEndpoint, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("PRIVATE-TOKEN", token)

		// Make the HTTP request
		client := &http.Client{}
		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()

		// Print the response body
		responseBody, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}

		var versions []*Release
		err = json.Unmarshal(responseBody, &versions)
		if err != nil {
			return nil, err
		}

		return versions[0], nil
	}

	steps = append(steps, ahoy_targets.NewTarget(
		fmt.Sprintf("%s_scripts", tmc.Name),
		ahoy_targets.WithSrcs(map[string][]string{"module": steps[len(steps)-1].Outs}),
		ahoy_targets.WithTargetScript("deploy", &ahoy_targets.TargetScript{
			Run: func(target *ahoy_targets.Target, runCtx *ahoy_targets.RuntimeContext) error {
				// GitLab personal access token with "api" scope
				token := "YOUR_GITLAB_API_TOKEN"

				// GitLab project ID where the module will be uploaded
				projectID := tmc.GitlabProject

				// Path to the Terraform module directory
				var archive *os.File
				zipOut := filepath.Join(target.Cwd, target.Outs[0])
				if a, err := os.Create(zipOut); err != nil {
					return nil
				} else {
					archive = a
				}

				zipWriter := zip.NewWriter(archive)

				for _, src := range target.Srcs["module"] {
					if f1, err := os.Open(src); err != nil {
						return nil
					} else if w1, err := zipWriter.Create(src); err != nil {
						return err
					} else if _, err := io.Copy(w1, f1); err != nil {
						return err
					} else {
						f1.Close()
					}
				}

				zipWriter.Close()
				archive.Close()

				// GitLab API endpoint for uploading a file
				apiEndpoint := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/packages/terraform/modules/%s/aws/%s/file", projectID, tmc.ModuleName, runCtx.Tag)

				// Create a new multipart form
				requestBody := &bytes.Buffer{}
				writer := multipart.NewWriter(requestBody)

				// Add the Terraform module directory as a file to the multipart form
				file, err := os.Open(zipOut)
				if err != nil {
					return err
				}
				defer file.Close()

				if filePart, err := writer.CreateFormFile("file", zipOut); err != nil {
					return err
				} else if _, err = io.Copy(filePart, file); err != nil {
					return err
				}

				// Close the multipart form
				err = writer.Close()
				if err != nil {
					return err
				}

				// Create a new HTTP request with the multipart form as the body
				request, err := http.NewRequest(http.MethodPost, apiEndpoint, requestBody)
				if err != nil {
					return err
				}
				request.Header.Set("Content-Type", writer.FormDataContentType())
				request.Header.Set("PRIVATE-TOKEN", token)

				// Make the HTTP request
				client := &http.Client{}
				response, err := client.Do(request)
				if err != nil {
					return err
				}
				defer response.Body.Close()

				// Print the response body
				_, err = ioutil.ReadAll(response.Body)
				if err != nil {
					return err
				}

				return nil
			},
		}),

		ahoy_targets.WithTargetScript("remove", &ahoy_targets.TargetScript{
			Run: func(target *ahoy_targets.Target, runCtx *ahoy_targets.RuntimeContext) error {
				// // GitLab personal access token with "api" scope
				// token := "YOUR_GITLAB_API_TOKEN"

				// // GitLab project ID where the module will be uploaded
				// projectID := tmc.GitlabProject

				// // GitLab API endpoint for removing a package
				// apiEndpoint := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/packages/%s", projectID, moduleId)

				// // Create a new HTTP request with the multipart form as the body
				// request, err := http.NewRequest(http.MethodDelete, apiEndpoint, nil)
				// if err != nil {
				// 	return err
				// }
				// request.Header.Set("PRIVATE-TOKEN", token)

				// // Make the HTTP request
				// client := &http.Client{}
				// response, err := client.Do(request)
				// if err != nil {
				// 	return err
				// }
				// defer response.Body.Close()

				// // Print the response body
				// responseBody, err := ioutil.ReadAll(response.Body)
				// if err != nil {
				// 	return err
				// }
				// fmt.Println(string(responseBody))

				return nil
			},
		}),
		ahoy_targets.WithTargetScript("current", &ahoy_targets.TargetScript{
			Run: func(target *ahoy_targets.Target, runCtx *ahoy_targets.RuntimeContext) error {
				current, err := getLatestVersion(target, runCtx)
				if err == nil {
					fmt.Printf("Current version is %s (%s)", current.Version, current.Id)
				}
				return err
			},
		}),
	))

	return steps, nil
}
