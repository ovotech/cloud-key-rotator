package crypt

import (
	b64 "encoding/base64"
	"errors"
	"os"
	"strings"

	enc "github.com/ovotech/mantle/crypt"
	"golang.org/x/crypto/openpgp"
)

//EncryptedServiceAccountKey uses github.com/ovotech/mantle to encrypt the
// key string that's passed in
func EncryptedServiceAccountKey(key, kmsKey string) (encKey []byte, err error) {
	const singleLine = false
	const disableValidation = true

	var decodedKey []byte
	if decodedKey, err = b64.StdEncoding.DecodeString(key); err != nil {
		return
	}

	return enc.CipherBytesFromPrimitives([]byte(decodedKey), singleLine,
		disableValidation, "", "", "", "", kmsKey), nil
}

//CommitSignKey creates an openPGP Entity based on a user's name, email,
//armoredKeyRing and passphrase for the key ring. This commitSignKey can then
//be used to GPG sign Git commits
func CommitSignKey(name, email, passphrase string) (entity *openpgp.Entity,
	err error) {
	if passphrase == "" {
		err = errors.New("ArmouredKeyRing passphrase must not be empty")
		return
	}
	var reader *os.File
	if reader, err = os.Open("/etc/cloud-key-rotator/akr.asc"); err != nil {
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
