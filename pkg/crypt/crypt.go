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

package crypt

import (
	"errors"
	"os"
	"strings"

	enc "github.com/ovotech/mantle/crypt"
	"golang.org/x/crypto/openpgp"
)

//EncryptedServiceAccountKey uses github.com/ovotech/mantle to encrypt the
// key string that's passed in
func EncryptedServiceAccountKey(key, kmsKey string) (encKey []byte) {
	const singleLine = false
	const disableValidation = true
	return enc.CipherBytesFromPrimitives([]byte(key), singleLine,
		disableValidation, "", "", "", "", kmsKey, enc.GcpKms{})
}

//CommitSignKey creates an openPGP Entity based on a user's name, email,
//armoredKeyRing and passphrase for the key ring. This commitSignKey can then
//be used to GPG sign Git commits
func CommitSignKey(name, email, passphrase, path string) (entity *openpgp.Entity,
	err error) {
	if len(passphrase) == 0 {
		err = errors.New("ArmouredKeyRing passphrase must not be empty")
		return
	}
	if len(path) == 0 {
		path = "/etc/cloud-key-rotator/akr.asc"
	}
	var reader *os.File
	if reader, err = os.Open(path); err != nil {
		if reader, err = os.Open("./akr.asc"); err != nil {
			return
		}
	}
	var entityList openpgp.EntityList
	if entityList, err = openpgp.ReadArmoredKeyRing(reader); err != nil {
		return
	}
	_, ok := entityList[0].Identities[strings.Join([]string{name, " <", email, ">"}, "")]
	if !ok {
		err = errors.New("Failed to add Identity to EntityList")
	}
	if err = entityList[0].PrivateKey.Decrypt([]byte(passphrase)); err != nil {
		return
	}
	entity = entityList[0]
	return
}
