package main

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvInput(t *testing.T) {

	// Ideally we wouldn't be messing with the overall test environment
	// But I don't see a way to have a go subprocess with its own env

	// Good parameters
	os.Setenv("DRONE_WORKSPACE", "/dev/null")
	os.Setenv("PLUGIN_AE_ENVIRONMENT", `{"key1":"value1", "key2":"value2"}`)
	os.Setenv("PLUGIN_SUB_COMMANDS", "do,this,now,please")

	// Bad parameter
	os.Setenv("PLUGIN_ADDL_ARGS", "stringthatshouldbejson")

	vargsBad := GAE{}
	workspaceBad := ""
	err := configFromEnv(&vargsBad, &workspaceBad)
	assert.EqualError(t, err, "could not parse param addl_args into a map[string]string")

	// Fix the bad param
	os.Setenv("PLUGIN_ADDL_ARGS", "")

	vargsGood := GAE{}
	workspaceGood := ""
	err = configFromEnv(&vargsGood, &workspaceGood)

	assert.Equal(t, "/dev/null", workspaceGood)

	desiredAEEnv := map[string]string{"key1": "value1", "key2": "value2"}
	assert.True(t, reflect.DeepEqual(vargsGood.AEEnv, desiredAEEnv))

	desiredSubCommands := []string{"do", "this", "now", "please"}
	assert.True(t, reflect.DeepEqual(vargsGood.SubCommands, desiredSubCommands))

	// Test unset variable
	assert.Equal(t, vargsGood.Version, "")

}

func TestValidateVargs(t *testing.T) {

	vargs := GAE{
		Token:   "mytoken",
		Project: "myproject",
		Action:  "dostuff",
	}
	assert.NoError(t, validateVargs(&vargs))

	vargs = GAE{
		Project: "myproject",
		Action:  "dostuff",
	}
	assert.EqualError(t, validateVargs(&vargs), "missing required param: token")

	vargs = GAE{
		Token:   "mytoken",
		Project: "myproject",
	}
	assert.EqualError(t, validateVargs(&vargs), "missing required param: action")

	vargs = GAE{
		Token:  "brokentoken",
		Action: "dostuff",
	}
	assert.EqualError(t, validateVargs(&vargs), "project id not found in token or param")

	vargs = GAE{
		Token:  `{"project_id": "my-gcp-project"}`,
		Action: "dostuff",
	}
	assert.NoError(t, validateVargs(&vargs))

	vargs = GAE{
		Token:   `{"project_id": "my-gcp-project"}`,
		Project: "my-other-project",
		Action:  "dostuff",
	}
	assert.NoError(t, validateVargs(&vargs))
	// Project field overrides token
	assert.Equal(t, "my-other-project", vargs.Project)
}

/*
//This works in regular go, but not under go test for some reason
//plugin.MustParse() panics due to stdin EOF
func TestStdinInput(t *testing.T) {

	tmpfile, _ := ioutil.TempFile("", "stdintest")
	tmpfileName := tmpfile.Name()
	defer os.Remove(tmpfileName)
	tmpfile.Write([]byte(`{"workspace":{"path":"/dev/null"}}`))
	tmpfile.Close()

	fakeStdin, err := os.Open(tmpfileName)
	if err != nil {
		t.Error(err)
	}
	//os.Stdin.Close()
	os.Stdin = fakeStdin

	vargs := GAE{}
	workspace := ""
	configFromStdin(&vargs, &workspace)

	assert.Equal(t, "/dev/null", workspace)
}
*/
