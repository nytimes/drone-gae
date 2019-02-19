package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
)

func removeOldVersions(runner *Environ, workspace, service string, vargs GAE) error {
	var versionJSON bytes.Buffer
	sout := runner.stdout
	runner.stdout = &versionJSON
	// look  up existing versions for given service ordered by create time desc
	err := runner.Run(vargs.GCloudCmd, "app", "versions", "list",
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
