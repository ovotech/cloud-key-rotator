// Copyright 2019 OVO Technology
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package location

import (
	"fmt"
	"strings"
	"time"

	circleci "github.com/jszwedko/go-circleci"
	"github.com/ovotech/cloud-key-rotator/pkg/cred"
	"github.com/ovotech/cloud-key-rotator/pkg/log"
)

//CircleCI type
type CircleCI struct {
	UsernameProject string
	KeyIDEnvVar     string
	KeyEnvVar       string
}

var logger = log.StdoutLogger().Sugar()

//updateCircleCI updates the circleCI environment variable by deleting and
//then creating it again with the new key
func (circle CircleCI) Write(serviceAccountName string, keyWrapper KeyWrapper, creds cred.Credentials) (updated UpdatedLocation, err error) {
	logger.Info("Starting CircleCI env var updates")
	client := &circleci.Client{Token: creds.CircleCIAPIToken}
	provider := keyWrapper.KeyProvider

	var keyEnvVar string
	var idValue bool
	if keyEnvVar, err = getVarNameFromProvider(provider, circle.KeyEnvVar, idValue); err != nil {
		return
	}

	var keyIDEnvVar string
	idValue = true
	if keyIDEnvVar, err = getVarNameFromProvider(provider, circle.KeyIDEnvVar, idValue); err != nil {
		return
	}

	splitUsernameProject := strings.Split(circle.UsernameProject, "/")
	username := splitUsernameProject[0]
	project := splitUsernameProject[1]

	if len(keyIDEnvVar) > 0 {
		if err = updateCircleCIEnvVar(username, project, keyIDEnvVar, keyWrapper.KeyID, client); err != nil {
			return
		}
	}

	if err = updateCircleCIEnvVar(username, project, keyEnvVar, keyWrapper.Key, client); err != nil {
		return
	}

	updated = UpdatedLocation{
		LocationType: "CircleCI",
		LocationURI:  circle.UsernameProject,
		LocationIDs:  []string{keyIDEnvVar, keyEnvVar}}

	return updated, nil
}

func updateCircleCIEnvVar(username, project, envVarName, envVarValue string, client *circleci.Client) (err error) {
	if err = verifyCircleCiEnvVar(username, project, envVarName, client); err != nil {
		return
	}
	if err = client.DeleteEnvVar(username, project, envVarName); err != nil {
		return
	}
	logger.Infof("Deleted CircleCI env var: %s from %s/%s", envVarName, username, project)
	if _, err = client.AddEnvVar(username, project, envVarName, envVarValue); err != nil {
		return
	}
	logger.Infof("Added CircleCI env var: %s to %s/%s", envVarName, username, project)
	return verifyCircleCiEnvVar(username, project, envVarName, client)
}

// Functions related to verifying CircleCI build, e.g. after changing another source such as GitHub, rather than updating credentials in Circle

//verifyCircleCIJobSuccess uses the specified gitHash to track down the circleCI
//build number, which it then uses to determine the status of the circleCI build
func verifyCircleCIJobSuccess(orgRepo, gitHash, circleCIDeployJobName, circleCIAPIToken string) (err error) {
	client := &circleci.Client{Token: circleCIAPIToken}
	splitOrgRepo := strings.Split(orgRepo, "/")
	org := splitOrgRepo[0]
	repo := splitOrgRepo[1]
	var targetBuildNum int
	if targetBuildNum, err = obtainBuildNum(org, repo, gitHash, circleCIDeployJobName,
		client); err != nil {
		return
	}
	return checkForJobSuccess(org, repo, targetBuildNum, client)
}

//checkForJobSuccess polls the circleCI API until the build is successful or
//failed, or a timeout is reached, whichever happens first
func checkForJobSuccess(org, repo string, targetBuildNum int, client *circleci.Client) (err error) {
	checkAttempts := 0
	checkLimit := 60
	checkInterval := 5 * time.Second
	logger.Infof("Polling CircleCI for status of build: %d", targetBuildNum)
	for {
		var build *circleci.Build
		if build, err = client.GetBuild(org, repo, targetBuildNum); err != nil {
			return
		}
		if build.Status == "success" {
			logger.Infof("Detected success of CircleCI build: %d", targetBuildNum)
			break
		} else if build.Status == "failed" {
			return fmt.Errorf("CircleCI job: %d has failed", targetBuildNum)
		}
		checkAttempts++
		if checkAttempts == checkLimit {
			return fmt.Errorf("Unable to verify CircleCI job was a success: https://circleci.com/gh/%s/%s/%d",
				org, repo, targetBuildNum)
		}
		time.Sleep(checkInterval)
	}
	return
}

//obtainBuildNum gets the number of the circleCI build by matching up the gitHash
func obtainBuildNum(org, repo, gitHash, circleCIDeployJobName string, client *circleci.Client) (targetBuildNum int, err error) {
	checkAttempts := 0
	checkLimit := 60
	checkInterval := 5 * time.Second
	for {
		var builds []*circleci.Build
		if builds, err = client.ListRecentBuildsForProject(org, repo, "master",
			"running", -1, 0); err != nil {
			return
		}
		targetBuildNum = buildNumFromRecentBuilds(builds, gitHash, circleCIDeployJobName)
		if targetBuildNum > 0 {
			break
		}
		checkAttempts++
		if checkAttempts == checkLimit {
			err = fmt.Errorf("Unable to determine CircleCI build number from target job name: %s",
				circleCIDeployJobName)
			return
		}
		time.Sleep(checkInterval)
	}
	return
}

//buildNumFromRecentBuilds returns an int that represents the number of a
// build that contains a job of the specified name
// The GitHash is used to ensure the correct build is selected
func buildNumFromRecentBuilds(builds []*circleci.Build, gitHash, circleCIDeployJobName string) (targetBuildNum int) {
	for _, build := range builds {
		logger.Infof("Checking for target job in CircleCI build: %d", build.BuildNum)
		if build.VcsRevision == gitHash &&
			build.BuildParameters["CIRCLE_JOB"] == circleCIDeployJobName {
			targetBuildNum = build.BuildNum
			return
		}
	}
	return
}

func verifyCircleCiEnvVar(username, project, envVarName string, client *circleci.Client) (err error) {
	var exists bool
	var envVars []circleci.EnvVar
	if envVars, err = client.ListEnvVars(username, project); err != nil {
		return
	}
	for _, envVar := range envVars {
		if envVar.Name == envVarName {
			exists = true
			break
		}
	}
	if exists {
		logger.Infof("Verified CircleCI env var: %s on %s/%s",
			envVarName, username, project)
	} else {
		err = fmt.Errorf("CircleCI env var: %s not detected on %s/%s",
			envVarName, username, project)
		return
	}
	return
}
