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
	"log"

	"github.com/Sectorbob/mlab-ns2/gae/ns/digest"
	"github.com/mongodb/go-client-mongodb-atlas/mongodbatlas"
	"github.com/ovotech/cloud-key-rotator/pkg/cred"
)

// Atlas type
type Atlas struct {
	BucketName string
	ObjectName string
	FileType   string
}

func newClient(publicKey, privateKey string) (*mongodbatlas.Client, error) {

	//Setup a transport to handle digest
	transport := digest.NewTransport(publicKey, privateKey)

	//Initialize the client
	client, err := transport.Client()
	if err != nil {
		return nil, err
	}

	//Initialize the MongoDB Atlas API Client.
	return mongodbatlas.NewClient(client), nil
}

func (atlas Atlas) Write(serviceAccountName string, keyWrapper KeyWrapper, 
	creds cred.Credentials) (updated UpdatedLocation, err error) {

	var err error
	var client *mongodbatlas.Client
	if client, err = newClient(creds.AtlasKeys.PublicKey, creds.AtlasKeys.PrivateKey); err != nil {
		return
	}

	var ear *mongodbatlas.EncryptionAtRest
	if ear, _, err = client.EncryptionsAtRest.Get(context.Background(), projectID); err != nil {
		return
	}

	provider := keyWrapper.KeyProvider

	switch provider {
	case "gcp":
		writeGcp()
	case "aws":
		writeAws(client, keyWrapper.KeyID, keyWrapper.Key )
	} 
}

func writeGcp() {

}

func writeAws(client *mongodbatlas.Client, accessKeyID, secretAccessKey string) (err error) {
	createRequest := &mongodbatlas.EncryptionAtRest{
		GroupID: projectID,
		AwsKms: mongodbatlas.AwsKms{
			AccessKeyID:     accessKeyID,
			SecretAccessKey: secretAccessKey,
		},
	}
	_, _, err = client.EncryptionsAtRest.Create(context.Background(), createRequest); err != nil {
	return
}

