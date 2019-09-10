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

package config

import (
	"bytes"
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/ovotech/cloud-key-rotator/pkg/cred"
	"github.com/ovotech/cloud-key-rotator/pkg/location"
	"github.com/spf13/viper"
)

//Config type
type Config struct {
	IncludeAwsUserKeys              bool
	IncludeInactiveKeys             bool
	Datadog                         Datadog
	DatadogAPIKey                   string
	RotationMode                    bool
	CloudProviders                  []CloudProvider
	AccountFilter                   Filter
	AccountKeyLocations             []KeyLocations
	Credentials                     cred.Credentials
	DefaultRotationAgeThresholdMins int
	EnableKeyAgeLogging             bool
}

//CloudProvider type
type CloudProvider struct {
	Name    string
	Project string
	Self    string
}

//Datadog type
type Datadog struct {
	MetricEnv  string
	MetricTeam string
	MetricName string
}

//Filter type
type Filter struct {
	Mode     string
	Accounts []ProviderServiceAccounts
}

//KeyLocations type
type KeyLocations struct {
	RotationAgeThresholdMins int
	ServiceAccountName       string
	CircleCI                 []location.CircleCI
	GCS                      []location.Gcs
	GitHub                   location.GitHub
	Gocd                     []location.Gocd
	K8s                      []location.K8s
}

//ProviderServiceAccounts type
type ProviderServiceAccounts struct {
	Provider         CloudProvider
	ProviderAccounts []string
}

const envVarPrefix = "ckr"

//GetConfig returns the application config
func GetConfig(configPath string) (c Config, err error) {
	viper.AutomaticEnv()
	viper.SetEnvPrefix(envVarPrefix)
	viper.AddConfigPath(configPath)
	viper.SetEnvPrefix("ckr")
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.ReadInConfig()
	if err = viper.Unmarshal(&c); err != nil {
		return
	}
	if !viper.IsSet("cloudProviders") {
		err = errors.New("cloudProviders is not set")
		return
	}
	return
}

// GetConfigFromAWSSecretManager grabs the cloud-key-rotator's config from
// AWS Secret Manager
func GetConfigFromAWSSecretManager(secretName, configType string) (c Config, err error) {
	var secret string
	if secret, err = getSecret(secretName); err != nil {
		return
	}
	if len(secret) == 0 {
		return c, errors.New("Unable to obtain secret value. Check user permissions and secret name")
	}
	viper.SetConfigType(configType)
	viper.ReadConfig(bytes.NewBufferString(secret))
	err = viper.Unmarshal(&c)
	return
}

func getSecret(secretName string) (secretString string, err error) {
	//Create a Secrets Manager client
	svc := secretsmanager.New(session.New())
	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretName),
		VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
	}
	var result *secretsmanager.GetSecretValueOutput
	if result, err = svc.GetSecretValue(input); err != nil {
		return
	}
	secretString = *result.SecretString
	return
}
