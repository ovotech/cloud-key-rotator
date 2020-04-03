# Atlas (mongodb) Example

## Pre-requisites

In order to rotate a key that's stored in Atlas encryption-at-rest config, you'll need:

1. An Atlas mongodb project
2. Public and private keys of an Atlas user with permissions to update encryption-at-rest-config (make sure you have the public and private keys to
hand).

## Configuration

The Atlas integration can currently only update AWS keys. Please raise a PR for GCP.

### AWS

Example of config to rotate an AWS key:

```json
{
  "RotationMode": true,
  "CloudProviders": [
    {
      "Name": "aws"
    }
  ],
  "AccountFilter": {
    "Mode": "include",
    "Accounts": [
      {
        "Provider": {
          "Name": "aws"
        },
        "ProviderAccounts": [
          "my_aws_machine_user"
        ]
      }
    ]
  },
  "AccountKeyLocations": [
    {
      "ServiceAccountName": "my_aws_machine_user",
      "Atlas": [
        {
          "ProjectID": "atlas_project_id"
        }
      ]
    }
  ],
  "Credentials": {
    "AtlasKeys": {
      "PublicKey": "atlas_public_key",
      "PrivateKey": "atlas_private_key"
    }
  }
}
```