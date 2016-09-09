package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/drone/drone-plugin-go/plugin"
)

type GAE struct {
	// [ update, update_cron, update_indexes, etc.]
	Action      string            `json:"action"`
	AddlArgs    map[string]string `json:"addl_args"`
	Version     string            `json:"version"`
	Environment map[string]string `json:"env"`

	AppFile string `json:"app_file"`
	Project string `json:"project"`
	Dir     string `json:"dir"`

	Token     string `json:"token"`
	GCloudCmd string `json:"gcloud_cmd"`
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
		vargs.AppCfgCmd = "/go_appengine/appcfg.py"
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

	e := os.Environ()
	// if app dir is not the base, make sure we set the appropriate path
	path := workspace.Path
	if vargs.Dir != "" {
		path = filepath.Join(path, vargs.Dir)
	}
	runner := NewEnviron(workspace.Path, e, os.Stdout, os.Stderr)

	// setup gcloud with our service account so we can use it for an access token
	err = runner.Run(vargs.GCloudCmd, "auth", "activate-service-account", "--key-file", keyPath)
	if err != nil {
		return fmt.Errorf("error: %s\n", err)
	}

	// build initial args for appcfg command
	args := []string{
		"--oauth2_access_token", "$(" + vargs.GCloudCmd + " auth print-access-token)",
		"-A", vargs.Project,
	}

	// add a version if we've got one
	if vargs.Version != "" {
		args = append(args, "-V", vargs.Version)
	}

	// add any env variables
	if len(vargs.Environment) > 0 {
		for k, v := range vargs.Environment {
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

	// some commands in appcfg are weird and require the app file to be named
	// 'app.yaml'. If an app file is given and it does not equal that, we need
	// to move it there
	if vargs.AppFile != "app.yaml" {
		orig := filepath.Join(workspace.Path, vargs.Dir, vargs.AppFile)
		dest := filepath.Join(workspace.Path, vargs.Dir, "app.yaml")
		err = os.Rename(orig, dest)
		if err != nil {
			return fmt.Errorf("error moving app file: %s\n", err)
		}
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
