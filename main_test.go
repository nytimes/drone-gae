package main

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvInput(t *testing.T) {

	// Ideally we wouldn't be messing with the overall test environment
	// But I don't see a way to have a go subprocess with its own env

	// test ENV Vars
	os.Setenv("SECRET_VALUE", "abc123")

	// Good parameters
	os.Setenv("DRONE_WORKSPACE", "/dev/null")
	os.Setenv("PLUGIN_AE_ENVIRONMENT", `{"key1":"value1", "key2":"value2"}`)
	os.Setenv("PLUGIN_VARS", `{"key1":"$SECRET_VALUE", "key2":"value2"}`)
	os.Setenv("PLUGIN_SUB_COMMANDS", "do,this,now,please")
	os.Setenv("GAE_CREDENTIALS", "{}")

	// Bad parameter
	os.Setenv("PLUGIN_ADDL_ARGS", "stringthatshouldbejson")

	vargs := GAE{}
	workspace := ""
	err := configFromEnv(&vargs, &workspace)
	assert.EqualError(t, err, "could not parse param addl_args into a map[string]string")

	// Fix the bad param
	os.Setenv("PLUGIN_ADDL_ARGS", "")

	vargs = GAE{}
	workspace = ""
	err = configFromEnv(&vargs, &workspace)

	assert.Equal(t, "/dev/null", workspace)

	assert.Equal(t, "{}", vargs.Token)

	desiredAEEnv := map[string]string{"key1": "value1", "key2": "value2"}
	assert.True(t, reflect.DeepEqual(vargs.AEEnv, desiredAEEnv))

	desiredTemplateVars := map[string]interface{}{"key1": "abc123", "key2": "value2"}
	assert.True(t, reflect.DeepEqual(vargs.TemplateVars, desiredTemplateVars))

	desiredSubCommands := []string{"do", "this", "now", "please"}
	assert.True(t, reflect.DeepEqual(vargs.SubCommands, desiredSubCommands))

	// Test unset variable
	assert.Equal(t, vargs.Version, "")
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

	// sanitize version
	vargs = GAE{
		Token:   "mytoken",
		Project: "myproject",
		Action:  "dostuff",
		Version: "feature/PRJ-test.branch/name",
	}
	assert.NoError(t, validateVargs(&vargs))
	assert.Equal(t, "feature-prj-test-branch-name", vargs.Version)
}

func TestSetupFile(t *testing.T) {
	tests := []struct {
		name string

		givenContents string
		givenVars     map[string]interface{}

		wantError  bool
		wantOutput string
	}{
		{
			name: "happy path",
			givenContents: `app:
  yes: {{ .Yes }}`,
			givenVars: map[string]interface{}{
				"Yes": true,
			},

			wantError: false,
			wantOutput: `app:
  yes: true`,
		},
		{
			name: "vars but no references in template",
			givenContents: `app:
  yes: true`,
			givenVars: map[string]interface{}{
				"Yes": true,
			},

			wantError: false,
			wantOutput: `app:
  yes: true`,
		},
		{
			name: "no vars but references in template",
			givenContents: `app:
  yes: {{ .Yes }}`,
			givenVars: map[string]interface{}{},

			wantError: true,
		},
		{
			name: "vars but references wrong in template",
			givenContents: `app:
  yes: {{ .Yes }}`,
			givenVars: map[string]interface{}{
				"No": "no",
			},

			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tf, err := ioutil.TempFile(".", "test-setup")
			if err != nil {
				t.Fatalf("unable to create temp file: %s", err)
			}
			defer os.Remove(tf.Name())

			_, err = tf.Write([]byte(test.givenContents))
			if err != nil {
				t.Fatalf("unable to write to temp file: %s", err)
			}
			err = tf.Close()
			if err != nil {
				t.Fatalf("unable to close temp file: %s", err)
			}

			// run test, only testing templating. we know file rename works
			gotErr := setupFile("", GAE{TemplateVars: test.givenVars}, tf.Name(), tf.Name())

			gotOutput, err := ioutil.ReadFile(tf.Name())
			if err != nil {
				t.Fatalf("unable to read temp file: %s", err)
			}

			if test.wantError && gotErr == nil {
				t.Error("expected error but got none")
			}
			// no need to check payload
			if test.wantError {
				return
			}
			if !test.wantError && gotErr != nil {
				t.Errorf("expected not error but one: %s", gotErr)
			}

			if string(gotOutput) != test.wantOutput {
				t.Errorf("expected file contents:\n%q\n\ngot:\n\n%q",
					test.wantOutput, string(gotOutput))
			}
		})
	}

}
