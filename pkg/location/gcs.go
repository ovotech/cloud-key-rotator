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

	"cloud.google.com/go/storage"
	"github.com/ovotech/cloud-key-rotator/pkg/cred"
)

// Gcs type
type Gcs struct {
	BucketName string
	ObjectName string
	FileType   string
}

func (gcs Gcs) Write(serviceAccountName string, keyWrapper KeyWrapper, creds cred.Credentials) (updated UpdatedLocation, err error) {
	var key string
	if key, err = getKeyForFileBasedLocation(keyWrapper, gcs.FileType); err != nil {
		return
	}
	ctx := context.Background()
	var client *storage.Client
	if client, err = storage.NewClient(ctx); err != nil {
		return
	}
	bkt := client.Bucket(gcs.BucketName)
	obj := bkt.Object(gcs.ObjectName)
	w := obj.NewWriter(ctx)
	if _, err = fmt.Fprintf(w, key); err != nil {
		return
	}
	if err := w.Close(); err != nil {
		logger.Warn("Error encountered trying to close GCS Writer", err)
	}
	updated = UpdatedLocation{
		LocationType: "GCS",
		LocationURI:  gcs.BucketName,
		LocationIDs:  []string{gcs.ObjectName}}
	return
}
