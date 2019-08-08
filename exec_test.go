package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvironRun(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	e := &Environ{
		dir:    "/tmp",
		env:    []string{"A=1"},
		stdout: stdout,
		stderr: stderr,
	}

	err := e.Run("/bin/echo", "hello, ae")
	if assert.NoError(t, err) {
		assert.Equal(t, "hello, ae\n", stdout.String())
		assert.Equal(t, "", stderr.String())
	}
}

func TestRedaction(t *testing.T) {
	arg := "--oauth2_access_token hello -E CREDENTIALS:{\n \"name\": \"nameVal\", \n \"name\": \n \"nameVal\", \n \"name\": \"nameVal\"\n }"
	redacted := reRedact.ReplaceAllString(strings.Trim(fmt.Sprint(arg), "[]"), " $1 [redacted]")

	assert.Equal(t, " --oauth2_access_token  [redacted] -E CREDENTIALS: [redacted]", redacted)
}
