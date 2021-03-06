# SSM (Parameter Store) Example

## Pre-requisites

In order to rotate a key that's stored in SSM parameters, you'll need:

1. Auth for `cloud-key-rotator` to create and destroy keys, and write to the required SSM parameter(s).

## Configuration

```json
  "AccountKeyLocations": [
    {
      "ServiceAccountName": "my_aws_machine_user",
      "Ssm": [
        {
          "KeyIDParamName": "ssm_key_id_param_name",
          "KeyParamName": "ssm_key_param_name",
          "Region": "my_gcs_bucket_name"
        }
      ]
    }
  ]
```

### AWS

If `KeyIDParamName` and/or `KeyParamName` fields are ommitted, the default values for AWS will be used, `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` respectively, which you probably don't want.

If you want the key ID + key values to output in a file, you can do that by specifying `ConvertToFile`. The default format is `.ini` but you can override to `.json` using the `FileType` field:

```json
      "Ssm": [
        {
          "KeyIDParamName": "ssm_key_id_param_name",
          "KeyParamName": "ssm_key_param_name",
          "Region": "my_gcs_bucket_name",
          "ConvertToFile": true,
          "FileType": "json"
        }
      ]
```

### GCP

Fields `KeyIDParamName`, `ConvertToFile` and `FileType` aren't used for GCP as
service account keys are always stored as a single string/file.