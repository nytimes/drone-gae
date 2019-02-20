package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

func removeOldVersions(runner *Environ, workspace string, vargs GAE) error {
	// read in the app.yaml file to grab the module/service mame we just deployed
	appLoc := filepath.Join(workspace, vargs.Dir, "app.yaml")
	appFile, err := os.Open(appLoc)
	if err != nil {
		return fmt.Errorf("error: %s\n", err)
	}
	defer appFile.Close()
	var appStruct struct {
		Service string `yaml:"service"`
		Module  string `yaml:"module"`
	}
	err = yaml.NewDecoder(appFile).Decode(&appStruct)
	if err != nil {
		return fmt.Errorf("error: %s\n", err)
	}

	service := appStruct.Service
	if service == "" {
		service = appStruct.Module
	}

	// look up existing versions for given service ordered by create time desc
	var versionJSON bytes.Buffer
	sout := runner.stdout
	runner.stdout = &versionJSON
	err = runner.Run(vargs.GCloudCmd, "app", "versions", "list",
		"--service", service, "--project", vargs.Project,
		"--format", "json", "--sort-by", "~version.createTime", "--quiet")
	if err != nil {
		return fmt.Errorf("error: %s\n", err)
	}

	var results []struct {
		ID           string  `json:"id"`
		TrafficSplit float64 `json:"traffic_split"`
	}
	err = json.NewDecoder(&versionJSON).Decode(&results)
	if err != nil {
		return err
	}

	var toDelete []string
	for i, res := range results {
		// keep newer versions, the newly deployed version or anything that has traffic
		if i < vargs.MaxVersions || res.ID == vargs.Version || res.TrafficSplit > 0 {
			continue
		}
		toDelete = append(toDelete, res.ID)
	}

	if len(toDelete) == 0 {
		return nil
	}

	log.Printf("deleting %d versions: %s", len(toDelete), toDelete)

	runner.stdout = sout
	args := []string{"app", "versions", "delete",
		"--service", service, "--project", vargs.Project, "--quiet"}
	args = append(args, toDelete...)
	err = runner.Run(vargs.GCloudCmd, args...)
	if err != nil {
		return fmt.Errorf("error: %s\n", err)
	}

	return nil
}
