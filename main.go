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
	"regexp"
	"strconv"
	"strings"
	"text/template"

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
	// the value will be sanitized (lowercase, replace `/` and `.` with `-`)
	Version string `json:"version"`
	// AEEnv allows users to set additional environment variables with `appcfg.py -E`
	// in their App Engine environment. This can be useful for injecting
	// secrets from your Drone secret store. No effect with `gcloud` commands.
	AEEnv map[string]string `json:"ae_environment"`
	// SubCommands are optionally used with `gcloud app` Actions to produce
	// complex commands like `gcloud app instances delete ...`.
	SubCommands []string `json:"sub_commands"`
	// FlexImage tells the plugin where to pull the image from when deploying a Flexible
	// VM instance. Example value: 'gcr.io/nyt-games-dev/puzzles-sub:$COMMIT'
	FlexImage string `json:"flex_image"`
	// TemplateVars allows users to pass a set of key/values to be injected into the
	// various yaml configuration files. To use, the keys in this map must be referenced
	// in the yaml files with Go's templating syntax. For example, the key "ABC" would be
	// referenced with {{ .ABC }}.
	TemplateVars map[string]interface{} `json:"vars"`

	// AppFile is the name of the app.yaml file to use for this deployment. This field
	// is only required if your app.yaml file is not named 'app.yaml'. Sometimes it is
	// helpful to have a different `app.yaml` file per project for different environment
	// and autoscaling configurations.
	AppFile string `json:"app_file"`

	// MaxVersions is an optional value that can be used along with the "deploy" or
	// "update" actions. If set to a non-zero value, the plugin will look up the versions
	// of the deployed service and delete any older versions beyond the "max" value
	// provided. If any of the "older" versions that should be deleted are actually
	// serving traffic, they will not be deleted. This may result in the actual version
	// count being higher than the max listed here.
	MaxVersions int `json:"max_versions"`

	// CronFile is the name of the cron.yaml file to use for this deployment. This field
	// is only required if your cron.yaml file is not named 'cron.yaml'
	CronFile string `json:"cron_file"`

	// DispatchFile is the name of the dispatch.yaml file to use for this deployment. This field
	// is only required if your dispatch.yaml file is not named 'dispatch.yaml'.
	DispatchFile string `json:"dispatch_file"`

	// QueueFile is the name of the queue.yaml file to use for this deployment. This field
	// is only required if your queue.yaml file is not named 'queue.yaml'.
	QueueFile string `json:"queue_file"`

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

	vargs := GAE{}
	workspace := ""

	// Check what drone version we're running on
	if os.Getenv("DRONE_WORKSPACE") == "" { // 0.4
		err := configFromStdin(&vargs, &workspace)
		if err != nil {
			return err
		}
	} else { // 0.5+
		err := configFromEnv(&vargs, &workspace)
		if err != nil {
			return err
		}
	}

	err := validateVargs(&vargs)
	if err != nil {
		return err
	}

	keyPath := "/tmp/gcloud.json"

	// Trim whitespace, to forgive the vagaries of YAML parsing.
	vargs.Token = strings.TrimSpace(vargs.Token)

	// Write credentials to tmp file to be picked up by the 'gcloud' command.
	// This is inside the ephemeral plugin container, not on the host.
	err = ioutil.WriteFile(keyPath, []byte(vargs.Token), 0600)
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

	runner := NewEnviron(filepath.Join(workspace, vargs.Dir), os.Environ(),
		os.Stdout, os.Stderr)

	// setup gcloud with our service account so we can use it for an access token
	err = runner.Run(vargs.GCloudCmd, "auth", "activate-service-account", "--key-file", keyPath)
	if err != nil {
		return fmt.Errorf("error: %s\n", err)
	}

	// if gcloud app cmd, run it
	if gcloudCmds[vargs.Action] {
		err = runGcloud(runner, workspace, vargs)
	} else {
		// otherwise, do appcfg.py command
		err = runAppCfg(runner, workspace, vargs)
	}

	if err != nil {
		return err
	}

	// check if MaxVersions is supplied + deploy action
	if vargs.MaxVersions > 0 && (vargs.Action == "deploy" || vargs.Action == "update") {
		return removeOldVersions(runner, workspace, vargs)
	}

	return nil
}

func configFromStdin(vargs *GAE, workspace *string) error {
	// https://godoc.org/github.com/drone/drone-plugin-go/plugin
	workspaceInfo := plugin.Workspace{}
	plugin.Param("workspace", &workspaceInfo)
	plugin.Param("vargs", vargs)
	// Note this hangs if no cli args or input on STDIN
	plugin.MustParse()

	*workspace = workspaceInfo.Path

	return nil
}

// GAE struct has different json for these, so use an intermediate for the new drone format
type dummyGAE struct {
	AddlArgs     map[string]string      `json:"-"`
	AEEnv        map[string]string      `json:"-"`
	TemplateVars map[string]interface{} `json:"-"`
}

func configFromEnv(vargs *GAE, workspace *string) error {

	// drone plugin input format du jour:
	// http://readme.drone.io/plugins/plugin-parameters/

	// Strings
	vargs.Project = os.Getenv("PLUGIN_PROJECT")
	vargs.Action = os.Getenv("PLUGIN_ACTION")
	*workspace = os.Getenv("DRONE_WORKSPACE")
	vargs.Token = os.Getenv("GAE_CREDENTIALS") // secrets are not prefixed
	vargs.Version = os.Getenv("PLUGIN_VERSION")
	vargs.FlexImage = os.Getenv("PLUGIN_FLEX_IMAGE")
	vargs.AppFile = os.Getenv("PLUGIN_APP_FILE")
	vargs.CronFile = os.Getenv("PLUGIN_CRON_FILE")
	vargs.DispatchFile = os.Getenv("PLUGIN_DISPATCH_FILE")
	vargs.QueueFile = os.Getenv("PLUGIN_QUEUE_FILE")
	vargs.Dir = os.Getenv("PLUGIN_DIR")
	vargs.AppCfgCmd = os.Getenv("PLUGIN_APPCFG_CMD")
	vargs.GCloudCmd = os.Getenv("PLUGIN_GCLOUD_CMD")
	vargs.MaxVersions, _ = strconv.Atoi(os.Getenv("PLUGIN_MAX_VERSIONS"))

	// Maps
	dummyVargs := dummyGAE{}
	addlArgs := os.Getenv("PLUGIN_ADDL_ARGS")
	if addlArgs != "" {
		if err := json.Unmarshal([]byte(addlArgs), &dummyVargs.AddlArgs); err != nil {
			return fmt.Errorf("could not parse param addl_args into a map[string]string")
		}
		vargs.AddlArgs = dummyVargs.AddlArgs
	}

	AEEnv := os.Getenv("PLUGIN_AE_ENVIRONMENT")
	if AEEnv != "" {
		if err := json.Unmarshal([]byte(AEEnv), &dummyVargs.AEEnv); err != nil {
			return fmt.Errorf("could not parse param ae_environment into a map[string]string")
		}

		// expand any env vars in template variable values
		for k, v := range dummyVargs.AEEnv {
			if s := os.ExpandEnv(v); s != "" {
				dummyVargs.AEEnv[k] = os.ExpandEnv(v)
			}
		}
		vargs.AEEnv = dummyVargs.AEEnv
	}

	templateVars := os.Getenv("PLUGIN_VARS")
	if templateVars != "" {
		if err := json.Unmarshal([]byte(templateVars), &dummyVargs.TemplateVars); err != nil {
			return fmt.Errorf("could not parse param vars into a map[string]interface{}")
		}

		// expand any env vars in template variable values
		for k, v := range dummyVargs.TemplateVars {
			if v, ok := v.(string); ok {
				if s := os.ExpandEnv(v); s != "" {
					dummyVargs.TemplateVars[k] = os.ExpandEnv(v)
				}
			}
		}
		vargs.TemplateVars = dummyVargs.TemplateVars
	}

	// Lists: pity the fool whose values include commas
	vargs.AddlFlags = strings.Split(os.Getenv("PLUGIN_ADDL_FLAGS"), ",")
	vargs.SubCommands = strings.Split(os.Getenv("PLUGIN_SUB_COMMANDS"), ",")

	return nil
}

func validateVargs(vargs *GAE) error {

	if vargs.Token == "" {
		return fmt.Errorf("missing required param: token")
	}

	if vargs.Project == "" {
		vargs.Project = getProjectFromToken(vargs.Token)
		if vargs.Project == "" {
			return fmt.Errorf("project id not found in token or param")
		}
	}

	if vargs.Action == "" {
		return fmt.Errorf("missing required param: action")
	}

	if vargs.AppCfgCmd == "" {
		vargs.AppCfgCmd = "/go_appengine/appcfg.py"
	}

	if vargs.GCloudCmd == "" {
		vargs.GCloudCmd = "gcloud"
	}

	if vargs.Version != "" {
		re := regexp.MustCompile(`[/|.]`)
		v := re.ReplaceAllString(vargs.Version, "-")
		vargs.Version = strings.ToLower(v)
	}

	return nil
}

var gcloudCmds = map[string]bool{
	"deploy":    true,
	"services":  true,
	"versions":  true,
	"instances": true,
}

func runGcloud(runner *Environ, workspace string, vargs GAE) error {
	// add the action first (gcloud app X)
	args := []string{
		"app",
		vargs.Action,
	}

	// Add subcommands to we can make complex calls like
	// 'gcloud app services X Y Z ...'
	for _, cmd := range vargs.SubCommands {
		if len(cmd) > 0 {
			args = append(args, cmd)
		}
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
	if len(vargs.AddlArgs) > 0 {
		for k, v := range vargs.AddlArgs {
			args = append(args, k, v)
		}
	}

	// add any additional singleton flags
	if len(vargs.AddlFlags) > 0 {
		for _, v := range vargs.AddlFlags {
			if len(v) > 0 {
				args = append(args, v)
			}
		}
	}

	if err := setupAppFile(workspace, vargs); err != nil {
		return err
	}

	if err := setupCronFile(workspace, vargs); err != nil {
		return err
	}

	if err := setupDispatchFile(workspace, vargs); err != nil {
		return err
	}

	if err := setupQueueFile(workspace, vargs); err != nil {
		return err
	}

	err := runner.Run(vargs.GCloudCmd, args...)
	if err != nil {
		return fmt.Errorf("error: %s\n", err)
	}
	return nil
}

func runAppCfg(runner *Environ, workspace string, vargs GAE) error {
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
func setupAppFile(workspace string, vargs GAE) error {
	return setupFile(workspace, vargs, "app.yaml", vargs.AppFile)
}

// Useful for differentiating between prd and dev cron versions for GCP appengine
func setupCronFile(workspace string, vargs GAE) error {
	return setupFile(workspace, vargs, "cron.yaml", vargs.CronFile)
}

// Useful for differentiating between prd and dev dispatch versions for GCP appengine
func setupDispatchFile(workspace string, vargs GAE) error {
	return setupFile(workspace, vargs, "dispatch.yaml", vargs.DispatchFile)
}

// Useful for differentiating between prd and dev queue versions for GCP appengine
func setupQueueFile(workspace string, vargs GAE) error {
	return setupFile(workspace, vargs, "queue.yaml", vargs.QueueFile)
}

// setupFile is used to copy a user-supplied file to a GAE-expected file.
// gaeName is the file name that GAE uses (ex: app.yaml, cron.yaml, default.yaml)
// suppliedName is the name of the file that should be renamed (ex: stg-app.yaml)
// If any template variables are provided, the file will be parsed and executed as
// a text/template with the variables injected.
func setupFile(workspace string, vargs GAE, gaeName string, suppliedName string) error {
	// if no file given, give up
	if suppliedName == "" {
		return nil
	}
	dest := filepath.Join(workspace, vargs.Dir, gaeName)
	if suppliedName != gaeName {
		orig := filepath.Join(workspace, vargs.Dir, suppliedName)
		err := copyFile(dest, orig)
		if err != nil {
			return fmt.Errorf("error moving %q to %q: %s\n", suppliedName, gaeName, err)
		}
	}

	// now that the file is in the right spot, we can inject any available TemplateVars.
	blob, err := ioutil.ReadFile(dest)
	if err != nil {
		return fmt.Errorf("Error reading template: %s\n", err)
	}

	tmpl, err := template.New(gaeName).Option("missingkey=error").Parse(string(blob))
	if err != nil {
		return fmt.Errorf("Error parsing template: %s\n", err)
	}

	out, err := os.OpenFile(dest, os.O_TRUNC|os.O_RDWR, 0755)
	if err != nil {
		return fmt.Errorf("Error opening template: %s\n", err)
	}
	defer out.Close()

	err = tmpl.Execute(out, vargs.TemplateVars)
	if err != nil {
		return fmt.Errorf("Error executing template: %s\n", err)
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
