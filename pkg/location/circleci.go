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
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	circleci "github.com/jszwedko/go-circleci"
	"github.com/ovotech/cloud-key-rotator/pkg/cred"
	"github.com/ovotech/cloud-key-rotator/pkg/log"
)

// // EnvVarLister type to allow for mocking
// type EnvVarLister func(username, project string, client *circleci.Client) ([]circleci.EnvVar, error)

// // EnvVarDeleter type to allow for mocking
// type EnvVarDeleter func(username, project, envVarName string, client *circleci.Client) error

// // EnvVarAdder type to allow for mocking
// type EnvVarAdder func(username, project, envVarName, envVarValue string, client *circleci.Client) (*circleci.EnvVar, error)

type circleCIListCaller interface {
	list() ([]circleci.EnvVar, error)
	getUsername() string
	getProject() string
}

type circleCIDeleteCaller interface {
	delete() error
	getEnvVarName() string
	getUsername() string
	getProject() string
}

type circleCIAddCaller interface {
	add() (*circleci.EnvVar, error)
	getEnvVarName() string
	getUsername() string
	getProject() string
}

type circleCICallList struct {
	client   *circleci.Client
	project  string
	username string
}

type circleCICallDelete struct {
	client     *circleci.Client
	envVarName string
	project    string
	username   string
}

type circleCICallAdd struct {
	client      *circleci.Client
	envVarName  string
	envVarValue string
	project     string
	username    string
}

func (c circleCICallList) list() ([]circleci.EnvVar, error) {
	return c.client.ListEnvVars(c.username, c.project)
}

func (c circleCICallList) getUsername() string {
	return c.username
}

func (c circleCICallList) getProject() string {
	return c.project
}

func (c circleCICallDelete) delete() error {
	return c.client.DeleteEnvVar(c.username, c.project, c.envVarName)
}

func (c circleCICallDelete) getEnvVarName() string {
	return c.envVarName
}

func (c circleCICallDelete) getUsername() string {
	return c.username
}

func (c circleCICallDelete) getProject() string {
	return c.project
}

func (c circleCICallAdd) add() (*circleci.EnvVar, error) {
	return c.client.AddEnvVar(c.username, c.project, c.envVarName, c.envVarValue)
}

func (c circleCICallAdd) getEnvVarName() string {
	return c.envVarName
}

func (c circleCICallAdd) getUsername() string {
	return c.envVarName
}

func (c circleCICallAdd) getProject() string {
	return c.envVarName
}

// // listEnvVars of type EnvVarLister
// func listEnvVars(username, project string, client *circleci.Client) ([]circleci.EnvVar, error) {
// 	return client.ListEnvVars(username, project)
// }

// // deleteEnvVar of type EnvVarDeleter
// func deleteEnvVar(username, project, envVarName string, client *circleci.Client) error {
// 	return client.DeleteEnvVar(username, project, envVarName)
// }

// // addEnvVar of type EnvVarAdder
// func addEnvVar(username, project, envVarName, envVarValue string, client *circleci.Client) (*circleci.EnvVar, error) {
// 	return client.AddEnvVar(username, project, envVarName, envVarValue)
// }

//CircleCI type
type CircleCI struct {
	UsernameProject string
	KeyIDEnvVar     string
	KeyEnvVar       string
	Base64Decode    bool
}

var logger = log.StdoutLogger().Sugar()

//updateCircleCI updates the circleCI environment variable by deleting and
//then creating it again with the new key
func (circle CircleCI) Write(serviceAccountName string, keyWrapper KeyWrapper, creds cred.Credentials) (updated UpdatedLocation, err error) {
	splitUsernameProject := strings.Split(circle.UsernameProject, "/")
	username := splitUsernameProject[0]
	project := splitUsernameProject[1]
	logger.Infof("Starting CircleCI env var updates, username: %s, project: %s",
		username, project)
	client := &circleci.Client{Token: creds.CircleCIAPIToken}
	provider := keyWrapper.KeyProvider
	key := keyWrapper.Key
	// if configured, base64 decode the key (GCP return encoded keys)
	if circle.Base64Decode {
		var keyb []byte
		keyb, err = base64.StdEncoding.DecodeString(key)
		if err != nil {
			return
		}
		key = string(keyb)
	}

	circleCICallList := circleCICallList{client: client, project: project, username: username}

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

	if len(keyIDEnvVar) > 0 {
		circleCICallDelete := circleCICallDelete{client: client, envVarName: keyIDEnvVar, username: username, project: project}
		circleCICallAdd := circleCICallAdd{client: client, envVarName: keyIDEnvVar, envVarValue: keyWrapper.KeyID, username: username, project: project}
		if err = updateCircleCIEnvVar(circleCICallList, circleCICallDelete, circleCICallAdd); err != nil {
			return
		}
	}
	circleCICallDelete := circleCICallDelete{client: client, envVarName: keyEnvVar, username: username, project: project}
	circleCICallAdd := circleCICallAdd{client: client, envVarName: keyIDEnvVar, envVarValue: keyWrapper.KeyID, username: username, project: project}
	if err = updateCircleCIEnvVar(circleCICallList, circleCICallDelete, circleCICallAdd); err != nil {
		return
	}

	updated = UpdatedLocation{
		LocationType: "CircleCI",
		LocationURI:  circle.UsernameProject,
		LocationIDs:  []string{keyIDEnvVar, keyEnvVar}}

	return updated, nil
}

func updateCircleCIEnvVar(circleCICallList circleCIListCaller, circleCICallDelete circleCIDeleteCaller, circleCICallAdd circleCIAddCaller) (err error) {
	if err = verifyCircleCiEnvVar(circleCICallList, circleCICallDelete.getEnvVarName()); err != nil {
		return
	}
	if err = circleCICallDelete.delete(); err != nil {
		return
	}
	logger.Infof("Deleted CircleCI env var: %s from %s/%s", circleCICallDelete.getEnvVarName(), circleCICallDelete.getUsername(), circleCICallDelete.getProject())
	if _, err = circleCICallAdd.add(); err != nil {
		return
	}
	logger.Infof("Added CircleCI env var: %s to %s/%s", circleCICallAdd.getEnvVarName(), circleCICallAdd.getUsername(), circleCICallAdd.getProject())
	return verifyCircleCiEnvVar(circleCICallList, circleCICallAdd.getEnvVarName())
}

func verifyCircleCiEnvVar(circleCICallList circleCIListCaller, envVarName string) (err error) {
	var exists bool
	var envVars []circleci.EnvVar
	if envVars, err = circleCICallList.list(); err != nil {
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
			envVarName, circleCICallList.getUsername(), circleCICallList.getProject())
	} else {
		err = fmt.Errorf("CircleCI env var: %s not detected on %s/%s",
			envVarName, circleCICallList.getUsername(), circleCICallList.getProject())
		return
	}
	return
}

////////////////////////////////////////////////////////////////////////////////
//
// Functions related to verifying CircleCI build, e.g. after changing another source such as GitHub, rather than updating credentials in Circle
//
////////////////////////////////////////////////////////////////////////////////

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
