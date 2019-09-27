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

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/ovotech/cloud-key-rotator/cmd"
	"github.com/ovotech/cloud-key-rotator/pkg/config"
	"github.com/ovotech/cloud-key-rotator/pkg/log"
	"github.com/ovotech/cloud-key-rotator/pkg/rotate"
)

// MyEvent type
type MyEvent struct {
	Name string `json:"name"`
}

var logger = log.StdoutLogger().Sugar()

//HandleRequest allows cloud-key-rotator to be used in the Lambda program model
func HandleRequest(ctx context.Context, name MyEvent) (string, error) {
	var c config.Config
	var err error
	status := "fail"
	if c, err = config.GetConfigFromAWSSecretManager(
		getEnv("CKR_SECRET_CONFIG_NAME", "ckr-config"),
		getEnv("CKR_CONFIG_TYPE", "json")); err != nil {
		return status, err
	}
	if err = rotate.Rotate("", "", "", c); err == nil {
		status = "success"
	}
	return status, err
}

func CloudFunctionRequest(w http.ResponseWriter, r *http.Request) {
	var c config.Config
	var err error
	var bucketName string
	var ok bool
	bucketEnvVarName := "CKR_BUCKET_NAME"
	if bucketName, ok = os.LookupEnv(bucketEnvVarName); ok {
		logCloudFunctionError(w, fmt.Errorf("Env var: %s is required", bucketEnvVarName))
		return
	}
	if c, err = config.GetConfigFromGCS(
		bucketName,
		getEnv("CKR_SECRET_CONFIG_NAME", "ckr-config"),
		getEnv("CKR_CONFIG_TYPE", "json")); err != nil {
		logCloudFunctionError(w, err)
		return
	}
	if err = rotate.Rotate("", "", "", c); err == nil {
		logCloudFunctionError(w, err)
		return
	}
}

func logCloudFunctionError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(err.Error()))
	logger.Error(err)
}

func main() {
	if rotate.InLambda() {
		lambda.Start(HandleRequest)
	} else {
		cmd.Execute()
	}
}

//getEnv returns the value of the env var matching the key, if it exists, and
// the value of fallback otherwise
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
