# GCS Example

## Pre-requisites

In order to rotate a key that's stored in a GCS bucket, you'll need:

1. A GCS bucket.
2. Auth for `cloud-key-rotator` to create and destroy keys, and write to the required GCS bucket.

## Configuration

### AWS

If you're rotating AWS keys, you could specify something like this:

```json
  "AccountKeyLocations": [
    {
      "ServiceAccountName": "my_aws_machine_user",
      "Gcs": [
        {
          "BucketName": "my_gcs_bucket_name",
          "ObjectName": "key.ini"
        }
      ]
    }
  ]
```

For AWS keys, by default, the key and key ID will be delivered to your GCS bucket in .ini format, e.g.:

```ini
[default]
aws_access_key_id=AKIGJDFSSDGGG
aws_secret_access_key=efkmfmfT$@Ggfg
```

If you prefer a JSON file, you can override like so:

```json
      "Gcs": [
        {
          "BucketName": "my_gcs_bucket_name",
          "ObjectName": "key.json",
          "FileType": "json"
        }
      ]
```

For GCP keys, the key will be delivered to your GCS bucket in .json format, e.g.:

```json
{
  "name": "projects/[PROJECT-ID]/serviceAccounts/[SERVICE-ACCOUNT-EMAIL]/keys/[KEY-ID]",
  "privateKeyType": "TYPE_GOOGLE_CREDENTIALS_FILE",
  "privateKeyData": "[PRIVATE-KEY]",
  "validAfterTime": "[DATE]",
  "validBeforeTime": "[DATE]",
  "keyAlgorithm": "KEY_ALG_RSA_2048"
}
```