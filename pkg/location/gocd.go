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
	"context"
	"fmt"

	gocdclient "github.com/beamly/go-gocd/gocd"
	"github.com/ovotech/cloud-key-rotator/pkg/cred"
)

// Gocd type
type Gocd struct {
	EnvName     string
	KeyIDEnvVar string
	KeyEnvVar   string
}

func (gocd Gocd) Write(serviceAccountName string, keyWrapper KeyWrapper, creds cred.Credentials) (updated UpdatedLocation, err error) {
	keyIDEnvVarName := gocd.KeyIDEnvVar
	envName := gocd.EnvName
	// only support secure Gocd env vars
	secure := true
	if len(keyIDEnvVarName) > 0 {
		if err = updateGocdEnvVar(keyIDEnvVarName, envName, keyWrapper.KeyID,
			creds.GocdServer.Server, creds.GocdServer.Username, creds.GocdServer.Password, creds.GocdServer.SkipSslCheck,
			secure); err != nil {
			return
		}
	}
	if err = updateGocdEnvVar(gocd.KeyEnvVar, envName, keyWrapper.Key,
		creds.GocdServer.Server, creds.GocdServer.Username, creds.GocdServer.Password, creds.GocdServer.SkipSslCheck,
		secure); err != nil {
		return
	}

	updated = UpdatedLocation{
		LocationType: "Gocd",
		LocationURI:  gocd.EnvName,
		LocationIDs:  []string{gocd.KeyIDEnvVar, gocd.KeyEnvVar}}

	return updated, nil
}

// patchGocdEnvVar removes the existing env var, and creates a new one (with the same name), in a single
// EnvironmentPatchRequest
func patchGocdEnvVar(targetEnvVarName, envName, envValue string, secure bool, c *gocdclient.Client) (err error) {
	envVar := &gocdclient.EnvironmentVariable{Name: targetEnvVarName, Value: envValue, Secure: secure}
	patch := gocdclient.EnvironmentPatchRequest{EnvironmentVariables: &gocdclient.EnvironmentVariablesAction{
		Add:    []*gocdclient.EnvironmentVariable{envVar},
		Remove: []string{targetEnvVarName}}}
	_, _, err = c.Environments.Patch(context.Background(), envName, &patch)
	return err
}

// verifyGocdEnvVar verifies that the target env var name exists in the specified Gocd env,
// returning an error if not found
func verifyGocdEnvVar(targetEnvVarName, envName string, c *gocdclient.Client) (err error) {
	var env *gocdclient.Environment
	if env, _, err = c.Environments.Get(context.Background(), envName); err != nil {
		return
	}
	for _, envVar := range env.EnvironmentVariables {
		if envVar.Name == targetEnvVarName {
			return
		}
	}
	return fmt.Errorf("Env var: %s not found in environment: %s", targetEnvVarName, envName)
}

// updateGocdEnvVar verifies the specified env var already exists, updates the value, and verifies again
func updateGocdEnvVar(targetEnvVarName, envName, key, server, user, pass string, skipSslCheck, secure bool) (err error) {
	cfg := gocdclient.Configuration{
		Server:       server,
		SkipSslCheck: skipSslCheck,
		Username:     user,
		Password:     pass,
	}
	c := cfg.Client()
	if err = verifyGocdEnvVar(targetEnvVarName, envName, c); err != nil {
		return
	}
	if err = patchGocdEnvVar(targetEnvVarName, envName, key, secure, c); err != nil {
		return
	}
	logger.Infof("Deleted Gocd env var: %s from env: %s", targetEnvVarName, envName)
	logger.Infof("Added Gocd env var: %s in env: %s", targetEnvVarName, envName)
	if err = verifyGocdEnvVar(targetEnvVarName, envName, c); err != nil {
		return
	}
	return
}
