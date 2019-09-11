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
	b64 "encoding/base64"
	"fmt"
)

//UpdatedLocation type
type UpdatedLocation struct {
	LocationType string
	LocationURI  string
	LocationIDs  []string
}

//KeyWrapper type
type KeyWrapper struct {
	Key         string
	KeyID       string
	KeyProvider string
}

func getKeyForFileBasedLocation(keyWrapper KeyWrapper) (key string, err error) {
	if keyWrapper.KeyProvider == "aws" {
		key = fmt.Sprintf("[default]\naws_access_key_id = %s\naws_secret_access_key = %s", keyWrapper.KeyID, keyWrapper.Key)
	} else {
		var keyBytes []byte
		if keyBytes, err = b64.StdEncoding.DecodeString(keyWrapper.Key); err == nil {
			key = string(keyBytes)
		}
	}
	return
}
