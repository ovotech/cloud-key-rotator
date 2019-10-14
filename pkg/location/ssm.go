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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	awsSsm "github.com/aws/aws-sdk-go/service/ssm"
	"github.com/ovotech/cloud-key-rotator/pkg/cred"
)

// Ssm type
type Ssm struct {
	keyParamName   string
	keyIDParamName string
	region         string
	convertToJSON  bool
}

func (ssm Ssm) Write(serviceAccountName string, keyWrapper KeyWrapper, creds cred.Credentials) (updated UpdatedLocation, err error) {
	provider := keyWrapper.KeyProvider
	var key string

	if ssm.convertToJSON {
		if key, err = getKeyForFileBasedLocation(keyWrapper); err != nil {
			return
		}
	} else {
		key = keyWrapper.Key
	}

	var keyEnvVar string
	if keyEnvVar, err = getVarNameFromProvider(provider, ssm.keyParamName); err != nil {
		return
	}

	var keyIDEnvVar string
	if keyIDEnvVar, err = getVarNameFromProvider(provider, ssm.keyIDParamName); err != nil {
		return
	}

	svc := awsSsm.New(session.New())
	svc.Config.Region = aws.String(ssm.region)

	if len(keyIDEnvVar) > 0 {
		if err = updateSSMParameter(keyIDEnvVar, keyWrapper.KeyID, "String", *svc); err != nil {
			return
		}
	}
	if err = updateSSMParameter(keyEnvVar, key, "SecureString", *svc); err != nil {
		return
	}

	updated = UpdatedLocation{
		LocationType: "SSM",
		LocationURI:  ssm.region,
		LocationIDs:  []string{keyIDEnvVar, keyEnvVar}}
	return
}

func updateSSMParameter(paramName, paramValue, paramType string, svc awsSsm.SSM) (err error) {
	input := &awsSsm.PutParameterInput{
		Overwrite: aws.Bool(true),
		Name:      aws.String(paramName),
		Value:     aws.String(paramValue),
	}
	_, err = svc.PutParameter(input)
	return
}
