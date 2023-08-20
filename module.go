package terraform

// import (
// 	"archive/zip"
// 	"bytes"
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"io/ioutil"
// 	"mime/multipart"
// 	"net/http"
// 	"os"
// 	"path/filepath"
// 	"regexp"
// 	"strings"

// 	git "github.com/go-git/go-git/v5"
// 	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
// 	getr "github.com/hashicorp/go-getter/v2"
// 	environs "github.com/zen-io/zen-core/environments"
// 	zen_targets "github.com/zen-io/zen-core/target"
// 	files "github.com/zen-io/zen-target-files"
// )

// type TerraformModuleConfig struct {
// 	Name          string                           `mapstructure:"name" zen:"yes" desc:"Name for the target"`
// 	Description   string                           `mapstructure:"desc" zen:"yes" desc:"Target description"`
// 	Labels        []string                         `mapstructure:"labels" zen:"yes" desc:"Labels to apply to the targets"`
// 	Deps          []string                         `mapstructure:"deps" zen:"yes" desc:"Build dependencies"`
// 	PassEnv       []string                         `mapstructure:"pass_env" zen:"yes" desc:"List of environment variable names that will be passed from the OS environment, they are part of the target hash"`
// 	SecretEnv     []string                         `mapstructure:"secret_env" zen:"yes" desc:"List of environment variable names that will be passed from the OS environment, they are not used to calculate the target hash"`
// 	Env           map[string]string                `mapstructure:"env" zen:"yes" desc:"Key-Value map of static environment variables to be used"`
// 	Tools         map[string]string                `mapstructure:"tools" zen:"yes" desc:"Key-Value map of tools to include when executing this target. Values can be references"`
// 	Visibility    []string                         `mapstructure:"visibility" zen:"yes" desc:"List of visibility for this target"`
// 	Environments  map[string]*environs.Environment `mapstructure:"environments" zen:"yes" desc:"Deployment Environments"`
// 	Srcs          []string                         `mapstructure:"srcs"`
// 	Url           *string                          `mapstructure:"url"`
// 	Hashes        []string                         `mapstructure:"hashes" zen:"yes"`
// 	Username      *string                          `mapstructure:"username"`
// 	Password      *string                          `mapstructure:"password"`
// 	Headers       map[string]string                `mapstructure:"headers,remain"`
// 	GitlabProject string                           `mapstructure:"gitlab_project"`
// 	ModuleName    string                           `mapstructure:"module_name"`
// }

// type Release struct {
// 	Version string `json:"version"`
// 	Id      string `json:"id"`
// }

// func (tmc TerraformModuleConfig) GetTargets(tcc *zen_targets.TargetConfigContext) ([]*zen_targets.Target, error) {
// 	targets := make([]*zen_targets.Target, 0)

// 	if tmc.Url != nil {
// 		out := regexp.MustCompile(`([^\.]+)(?:\..*)?`).ReplaceAllString(filepath.Base(*tmc.Url), "$1")

// 		t := zen_targets.ToTarget(tmc)
// 		t.Outs = []string{out}
// 		t.Scripts = map[string]*zen_targets.TargetScript{
// 			"build": {
// 				Deps: tmc.Deps,
// 				Run: func(target *zen_targets.Target, runCtx *zen_targets.RuntimeContext) error {
// 					if strings.HasPrefix(*tmc.Url, "git") || strings.HasSuffix(*tmc.Url, ".git") {
// 						// Set up authentication
// 						opts := &git.CloneOptions{
// 							URL:      *tmc.Url,
// 							Progress: target,
// 						}

// 						if tmc.Username != nil && tmc.Password != nil {
// 							opts.Auth = &gitHttp.BasicAuth{
// 								Username: *tmc.Username,
// 								Password: *tmc.Password,
// 							}
// 						}

// 						// Clone the repository to a local directory
// 						if _, err := git.PlainClone(filepath.Join(target.Cwd, out), false, opts); err != nil {
// 							return err
// 						}
// 					} else if _, err := getr.GetAny(context.TODO(), filepath.Join(target.Cwd, out), *tmc.Url); err != nil {
// 						return err
// 					}

// 					return nil
// 				},
// 			},
// 		}
// 		targets = append(targets, t)
// 	} else {
// 		fc := files.FilegroupConfig{
// 			Name:    tmc.Name,
// 			Labels:  tmc.Labels,
// 			Deps:    tmc.Deps,
// 			Srcs:    tmc.Srcs,
// 			Flatten: true,
// 		}
// 		if fgTargets, err := fc.GetTargets(tcc); err != nil {
// 			return nil, err
// 		} else {
// 			targets = fgTargets
// 		}
// 	}

// 	var getLatestVersion = func(ctx *zen_targets.Target, runCtx *zen_targets.RuntimeContext) (*Release, error) {
// 		// GitLab personal access token with "api" scope
// 		token := "YOUR_GITLAB_API_TOKEN"

// 		// GitLab project ID where the module will be uploaded
// 		projectID := tmc.GitlabProject

// 		// GitLab API endpoint for uploading a file
// 		apiEndpoint := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/packages?package_type=terraform_module&package_name=%s&order_by=version&sort=desc", projectID, tmc.ModuleName)

// 		// Create a new HTTP request with the multipart form as the body
// 		request, err := http.NewRequest(http.MethodDelete, apiEndpoint, nil)
// 		if err != nil {
// 			return nil, err
// 		}
// 		request.Header.Set("PRIVATE-TOKEN", token)

// 		// Make the HTTP request
// 		client := &http.Client{}
// 		response, err := client.Do(request)
// 		if err != nil {
// 			return nil, err
// 		}
// 		defer response.Body.Close()

// 		// Print the response body
// 		responseBody, err := ioutil.ReadAll(response.Body)
// 		if err != nil {
// 			return nil, err
// 		}

// 		var versions []*Release
// 		err = json.Unmarshal(responseBody, &versions)
// 		if err != nil {
// 			return nil, err
// 		}

// 		return versions[0], nil
// 	}

// 	steps = append(steps, zen_targets.NewTarget(
// 		fmt.Sprintf("%s_scripts", tmc.Name),
// 		zen_targets.WithSrcs(map[string][]string{"module": steps[len(steps)-1].Outs}),
// 		zen_targets.WithTargetScript("deploy", &zen_targets.TargetScript{
// 			Run: func(target *zen_targets.Target, runCtx *zen_targets.RuntimeContext) error {
// 				// GitLab personal access token with "api" scope
// 				token := "YOUR_GITLAB_API_TOKEN"

// 				// GitLab project ID where the module will be uploaded
// 				projectID := tmc.GitlabProject

// 				// Path to the Terraform module directory
// 				var archive *os.File
// 				zipOut := filepath.Join(target.Cwd, target.Outs[0])
// 				if a, err := os.Create(zipOut); err != nil {
// 					return nil
// 				} else {
// 					archive = a
// 				}

// 				zipWriter := zip.NewWriter(archive)

// 				for _, src := range target.Srcs["module"] {
// 					if f1, err := os.Open(src); err != nil {
// 						return nil
// 					} else if w1, err := zipWriter.Create(src); err != nil {
// 						return err
// 					} else if _, err := io.Copy(w1, f1); err != nil {
// 						return err
// 					} else {
// 						f1.Close()
// 					}
// 				}

// 				zipWriter.Close()
// 				archive.Close()

// 				// GitLab API endpoint for uploading a file
// 				apiEndpoint := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/packages/terraform/modules/%s/aws/%s/file", projectID, tmc.ModuleName, runCtx.Tag)

// 				// Create a new multipart form
// 				requestBody := &bytes.Buffer{}
// 				writer := multipart.NewWriter(requestBody)

// 				// Add the Terraform module directory as a file to the multipart form
// 				file, err := os.Open(zipOut)
// 				if err != nil {
// 					return err
// 				}
// 				defer file.Close()

// 				if filePart, err := writer.CreateFormFile("file", zipOut); err != nil {
// 					return err
// 				} else if _, err = io.Copy(filePart, file); err != nil {
// 					return err
// 				}

// 				// Close the multipart form
// 				err = writer.Close()
// 				if err != nil {
// 					return err
// 				}

// 				// Create a new HTTP request with the multipart form as the body
// 				request, err := http.NewRequest(http.MethodPost, apiEndpoint, requestBody)
// 				if err != nil {
// 					return err
// 				}
// 				request.Header.Set("Content-Type", writer.FormDataContentType())
// 				request.Header.Set("PRIVATE-TOKEN", token)

// 				// Make the HTTP request
// 				client := &http.Client{}
// 				response, err := client.Do(request)
// 				if err != nil {
// 					return err
// 				}
// 				defer response.Body.Close()

// 				// Print the response body
// 				_, err = ioutil.ReadAll(response.Body)
// 				if err != nil {
// 					return err
// 				}

// 				return nil
// 			},
// 		}),

// 		zen_targets.WithTargetScript("remove", &zen_targets.TargetScript{
// 			Run: func(target *zen_targets.Target, runCtx *zen_targets.RuntimeContext) error {
// 				// // GitLab personal access token with "api" scope
// 				// token := "YOUR_GITLAB_API_TOKEN"

// 				// // GitLab project ID where the module will be uploaded
// 				// projectID := tmc.GitlabProject

// 				// // GitLab API endpoint for removing a package
// 				// apiEndpoint := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/packages/%s", projectID, moduleId)

// 				// // Create a new HTTP request with the multipart form as the body
// 				// request, err := http.NewRequest(http.MethodDelete, apiEndpoint, nil)
// 				// if err != nil {
// 				// 	return err
// 				// }
// 				// request.Header.Set("PRIVATE-TOKEN", token)

// 				// // Make the HTTP request
// 				// client := &http.Client{}
// 				// response, err := client.Do(request)
// 				// if err != nil {
// 				// 	return err
// 				// }
// 				// defer response.Body.Close()

// 				// // Print the response body
// 				// responseBody, err := ioutil.ReadAll(response.Body)
// 				// if err != nil {
// 				// 	return err
// 				// }
// 				// fmt.Println(string(responseBody))

// 				return nil
// 			},
// 		}),
// 		zen_targets.WithTargetScript("current", &zen_targets.TargetScript{
// 			Run: func(target *zen_targets.Target, runCtx *zen_targets.RuntimeContext) error {
// 				current, err := getLatestVersion(target, runCtx)
// 				if err == nil {
// 					fmt.Printf("Current version is %s (%s)", current.Version, current.Id)
// 				}
// 				return err
// 			},
// 		}),
// 	))

// 	return steps, nil
// }
