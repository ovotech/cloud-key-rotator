package location

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
	"github.com/ovotech/cloud-key-rotator/pkg/cred"
)

type Gcs struct {
	bucketName string
	objectName string
}

func (gcs Gcs) Write(serviceAccountName string, keyWrapper KeyWrapper, creds cred.Credentials) (updated UpdatedLocation, err error) {
	var key string
	if key, err = getKeyForFileBasedLocation(keyWrapper); err != nil {
		return
	}
	ctx := context.Background()
	var client *storage.Client
	if client, err = storage.NewClient(ctx); err != nil {
		return
	}
	bkt := client.Bucket(gcs.bucketName)
	obj := bkt.Object(gcs.objectName)
	w := obj.NewWriter(ctx)
	if _, err = fmt.Fprintf(w, key); err != nil {
		return
	}
	if err := w.Close(); err != nil {
		logger.Warn("Error encountered trying to close GCS Writer", err)
	}
	updated = UpdatedLocation{
		LocationType: "GCS",
		LocationURI:  gcs.bucketName,
		LocationIDs:  []string{gcs.objectName}}
	return
}
