package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/drone/drone-plugin-go/plugin"
)

type GAE struct {
	// Action is required and can be any action accepted by the `appcfg.py` or
	// `gcloud app` commands:
	//
	// appcfg.py (update, update_cron, update_indexes, set_default_version, etc.)
	// gcloud app (deploy, services, versions, etc.)
	Action string `json:"action"`
	// AddlArgs is a set of key-value pairs to allow users to pass along any
	// additional parameters to the `appcfg.py` command.
	AddlArgs map[string]string `json:"addl_args"`
	// AddlFlags is an array of flag parameters that do not have a value.
	AddlFlags []string `json:"addl_flags"`
	// Version is used to set the version of new deployments
	// or to alter existing deployments.
	Version string `json:"version"`
	// AEEnv allows users to set additional environment variables
	// in their App Engine environment. This can be useful for injecting
	// secrets from your Drone secret store.
	AEEnv map[string]string `json:"ae_environment"`
	// SubCommands are optionally used with `gcloud app` Actions to produce
	// complex commands like `gcloud app instances delete ...`.
	SubCommands []string `json:"sub_commands"`
	// FlexImage tells the plugin where to pull the image from when deploying a Flexible
	// VM instance. Example value: 'gcr.io/nyt-games-dev/puzzles-sub:$COMMIT'
	FlexImage string `json:"flex_image"`

	// AppFile is the name of the app.yaml file to use for this deployment. This field
	// is only required if your app.yaml file is not named 'app.yaml'. Sometimes it is
	// helpful to have a different `app.yaml` file per project for different environment
	// and autoscaling configurations.
	AppFile string `json:"app_file"`

	// CronFile is the name of the cron.yaml file to use for this deployment. This field
	// is only required if your cron.yaml file is not named 'cron.yaml'
	CronFile string `json:"cron_file"`

	// Dir points to the directory the application exists in. This is only required if
	// you application is not in the base directory.
	Dir string `json:"dir"`

	// Project is required. It should be the Google Cloud Project to deploy to.
	Project string `json:"project"`
	// Token is required and should contain the JSON key of a service account associated
	// with the Google Cloud project the user wishes to interact with.
	Token string `json:"token"`

	// GCloudCmd is an optional override for the location of the gcloud CLI tool. This
	// may be useful if using a custom image.
	GCloudCmd string `json:"gcloud_cmd"`
	// AppCfgCmd is an optional override for the location of the App Engine appcfg.py
	// tool. This may be useful if using a custom image.
	AppCfgCmd string `json:"appcfg_cmd"`
}

var (
	rev string
)

func main() {
	err := wrapMain()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func wrapMain() error {
	if rev == "" {
		rev = "[unknown]"
	}

	fmt.Printf("Drone GAE Plugin built from %s\n", rev)

	// https://godoc.org/github.com/drone/drone-plugin-go/plugin
	workspace := plugin.Workspace{}
	repo := plugin.Repo{}
	build := plugin.Build{}
	system := plugin.System{}
	vargs := GAE{}

	plugin.Param("workspace", &workspace)
	plugin.Param("repo", &repo)
	plugin.Param("build", &build)
	plugin.Param("system", &system)
	plugin.Param("vargs", &vargs)
	plugin.MustParse()

	// Check required params
	if vargs.Token == "" {
		return fmt.Errorf("missing required param: token")
	}

	if vargs.Project == "" {
		vargs.Project = getProjectFromToken(vargs.Token)
	}

	if vargs.Project == "" {
		return fmt.Errorf("missing required param: project")
	}

	if vargs.Action == "" {
		return fmt.Errorf("missing required param: action")
	}

	keyPath := "/tmp/gcloud.json"

	// Defaults

	if vargs.GCloudCmd == "" {
		vargs.GCloudCmd = "/google-cloud-sdk/bin/gcloud"
	}

	if vargs.AppCfgCmd == "" {
		vargs.AppCfgCmd = "/google-cloud-sdk/platform/google_appengine/appcfg.py"
	}

	// Trim whitespace, to forgive the vagaries of YAML parsing.
	vargs.Token = strings.TrimSpace(vargs.Token)

	// Write credentials to tmp file to be picked up by the 'gcloud' command.
	// This is inside the ephemeral plugin container, not on the host.
	err := ioutil.WriteFile(keyPath, []byte(vargs.Token), 0600)
	if err != nil {
		return fmt.Errorf("error writing token file: %s\n", err)
	}

	// Warn if the keyfile can't be deleted, but don't abort. We're almost
	// certainly running inside an ephemeral container, so the file will be
	// discarded when we're finished anyway.
	defer func() {
		err := os.Remove(keyPath)
		if err != nil {
			fmt.Printf("warning: error removing token file: %s\n", err)
		}
	}()

	runner := NewEnviron(filepath.Join(workspace.Path, vargs.Dir), os.Environ(),
		os.Stdout, os.Stderr)

	// setup gcloud with our service account so we can use it for an access token
	err = runner.Run(vargs.GCloudCmd, "auth", "activate-service-account", "--key-file", keyPath)
	if err != nil {
		return fmt.Errorf("error: %s\n", err)
	}

	// if gcloud app cmd, run it
	if found := gcloudCmds[vargs.Action]; found {
		return runGcloud(runner, workspace, vargs)
	}

	// otherwise, do appcfg.py command
	return runAppCfg(runner, workspace, vargs)

}

var gcloudCmds = map[string]bool{
	"deploy":    true,
	"services":  true,
	"versions":  true,
	"instances": true,
}

func runGcloud(runner *Environ, workspace plugin.Workspace, vargs GAE) error {
	// add the action first (gcloud app X)
	args := []string{
		"app",
		vargs.Action,
	}

	// Add subcommands to we can make complex calls like
	// 'gcloud app services X Y Z ...'
	for _, cmd := range vargs.SubCommands {
		args = append(args, cmd)
	}

	// add the app.yaml location
	args = append(args, "./app.yaml")

	// add a version if we've got one
	if vargs.Version != "" {
		args = append(args, "--version", vargs.Version)
	}

	if vargs.FlexImage != "" {
		args = append(args, "--image-url", vargs.FlexImage)
	}

	if len(vargs.Project) > 0 {
		args = append(args, "--project", vargs.Project)
	}

	// add flag to prevent interactive
	args = append(args, "--quiet")

	// add the remaining arguments
	for k, v := range vargs.AddlArgs {
		args = append(args, k, v)
	}
	for _, v := range vargs.AddlFlags {
		args = append(args, v)
	}

	if err := setupAppFile(workspace, vargs); err != nil {
		return err
	}

	if err := setupCronFile(workspace, vargs); err != nil {
		return err
	}

	err := runner.Run(vargs.GCloudCmd, args...)
	if err != nil {
		return fmt.Errorf("error: %s\n", err)
	}

	return nil
}

func runAppCfg(runner *Environ, workspace plugin.Workspace, vargs GAE) error {
	// get access token string to pass along to `appcfg.py`
	tokenCmd := exec.Command(vargs.GCloudCmd, "auth", "print-access-token")
	var accessToken bytes.Buffer
	tokenCmd.Stdout = &accessToken
	err := tokenCmd.Run()
	if err != nil {
		return fmt.Errorf("error creating access token: %s\n", err)
	}

	// build initial args for appcfg command
	args := []string{
		"--oauth2_access_token", accessToken.String(),
		"-A", vargs.Project,
	}

	// add a version if we've got one
	if vargs.Version != "" {
		args = append(args, "-V", vargs.Version)
	}

	// add any env variables
	if len(vargs.AEEnv) > 0 {
		for k, v := range vargs.AEEnv {
			args = append(args, "-E", k+":"+v)
		}
	}

	// add any additional variables
	if len(vargs.AddlArgs) > 0 {
		for k, v := range vargs.AddlArgs {
			args = append(args, k, v)
		}
	}

	// add action and current dir
	args = append(args, vargs.Action, ".")

	if err = setupAppFile(workspace, vargs); err != nil {
		return err
	}

	if err = setupCronFile(workspace, vargs); err != nil {
		return err
	}

	err = runner.Run(vargs.AppCfgCmd, args...)
	if err != nil {
		return fmt.Errorf("error: %s\n", err)
	}

	return nil
}

type token struct {
	ProjectID string `json:"project_id"`
}

func getProjectFromToken(j string) string {
	t := token{}
	err := json.Unmarshal([]byte(j), &t)
	if err != nil {
		return ""
	}
	return t.ProjectID
}

// some app engine commands are weird and require the app file to be named
// 'app.yaml'. If an app file is given and it does not equal that, we need
// to copy it there
func setupAppFile(workspace plugin.Workspace, vargs GAE) error {
	if vargs.AppFile != "app.yaml" && vargs.AppFile != "" {
		orig := filepath.Join(workspace.Path, vargs.Dir, vargs.AppFile)
		dest := filepath.Join(workspace.Path, vargs.Dir, "app.yaml")
		err := copyFile(dest, orig)
		if err != nil {
			return fmt.Errorf("error moving app file: %s\n", err)
		}
	}
	return nil
}

// Useful for differentiating between prd and dev cron versions for GCP appengine
func setupCronFile(workspace plugin.Workspace, vargs GAE) error {
	if vargs.CronFile != "cron.yaml" && vargs.CronFile != "" {
		orig := filepath.Join(workspace.Path, vargs.Dir, vargs.CronFile)
		dest := filepath.Join(workspace.Path, vargs.Dir, "cron.yaml")
		err := copyFile(dest, orig)
		if err != nil {
			return fmt.Errorf("error moving cron file: %s\n", err)
		}
	}
	return nil
}

func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	tmp, err := ioutil.TempFile(filepath.Dir(dst), "")
	if err != nil {
		return err
	}
	_, err = io.Copy(tmp, in)
	if err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err = tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err = os.Chmod(tmp.Name(), info.Mode()); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), dst)
}
