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
	"bytes"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"

	"gopkg.in/ini.v1"
)

// UpdatedLocation type
type UpdatedLocation struct {
	LocationType string
	LocationURI  string
	LocationIDs  []string
}

// KeyWrapper type
type KeyWrapper struct {
	Key         string
	KeyID       string
	KeyProvider string
}

type envVarDefaults struct {
	keyEnvVar   string
	keyIDEnvVar string
	fileType    string
}

var (
	defaultsMap = map[string]envVarDefaults{
		"aiven": {
			fileType:  "",
			keyEnvVar: "AIVEN_TOKEN",
		},
		"aws": {
			fileType:    "ini",
			keyEnvVar:   "AWS_SECRET_ACCESS_KEY",
			keyIDEnvVar: "AWS_ACCESS_KEY_ID",
		},
		"gcp": {
			fileType:    "b64",
			keyEnvVar:   "GCLOUD_SERVICE_KEY",
			keyIDEnvVar: "",
		},
	}
)

func getKeyForFileBasedLocation(keyWrapper KeyWrapper, suppliedFileType string) (key string, err error) {
	provider := keyWrapper.KeyProvider
	var fileType string
	if fileType, err = getFileTypeFromProvider(provider, suppliedFileType); err != nil {
		return
	}
	switch fileType {
	case "b64":
		var keyBytes []byte
		if keyBytes, err = b64.StdEncoding.DecodeString(keyWrapper.Key); err == nil {
			key = string(keyBytes)
		}
	case "ini":
		cfg := ini.Empty()
		section := "default"
		cfg.NewSection(section)
		if _, err = cfg.Section(section).NewKey("aws_access_key_id",
			keyWrapper.KeyID); err != nil {
			return
		}
		if _, err = cfg.Section(section).NewKey("aws_secret_access_key",
			keyWrapper.Key); err != nil {
			return
		}
		buf := new(bytes.Buffer)
		cfg.WriteTo(buf)
		key = buf.String()
	case "json":
		creds := map[string]string{
			"aws_access_key_id":     keyWrapper.KeyID,
			"aws_secret_access_key": keyWrapper.Key,
		}
		var jsonCreds []byte
		if jsonCreds, err = json.Marshal(creds); err != nil {
			return
		}
		key = string(jsonCreds)
	}
	return
}

func envVarDefaultsFromProvider(provider string) (envVarDefaults envVarDefaults, err error) {
	for k, v := range defaultsMap {
		if k == provider {
			envVarDefaults = v
			return
		}
	}
	err = fmt.Errorf("No default env var names available for provider: %s", provider)
	return
}

func getFileTypeFromProvider(provider, suppliedFileType string) (fileType string, err error) {
	if len(suppliedFileType) > 0 {
		fileType = suppliedFileType
	} else {
		if providerDefault, err := envVarDefaultsFromProvider(provider); err == nil {
			fileType = providerDefault.fileType
		}
	}
	return
}

func getVarNameFromProvider(provider, suppliedVarName string, idValue bool) (envName string, err error) {
	if len(suppliedVarName) > 0 {
		envName = suppliedVarName
	} else {
		var defaultEnvVar envVarDefaults
		if defaultEnvVar, err = envVarDefaultsFromProvider(provider); err != nil {
			return
		}
		if idValue {
			envName = defaultEnvVar.keyIDEnvVar
		} else {
			envName = defaultEnvVar.keyEnvVar
		}
	}
	return
}
