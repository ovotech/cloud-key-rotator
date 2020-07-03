# AWS

Whether you're rotating keys for AWS or GCP users/service-accounts, the config
you give to `cloud-key-rotator` are largely the same.

That said, here's a minimal config for AWS rotation, with keys stored in
CircleCI:

```json
{
  "RotationMode": true,
  "CloudProviders": [
    {
      "Name": "aws",
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
      "CircleCI": [
        {
          "UsernameProject": "my_org/my_repo"
        }
      ]
    }
  ],
  "Credentials": {
    "CircleCIAPIToken": "my_circle_ci_api_token"
  }
}
```

Note: this assumes you're setting your key/keyID values in CircleCI env vars
named `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`. You can override these
defaults, see the [CircleCI example](../locations/circleci/README.md).

If you're storing your keys in locations other than CircleCI, you'll need to
replace the struct:

```json
      "CircleCI": [
        {
          "UsernameProject": "my_org/my_repo"
        }
      ]
```

..with whatever location type you need. See the location-specific example
for help. Otherwise, the rest of the config can remain the same.

You can specify multiple `ProviderAccounts`, and you can set multiple
`AccountKeyLocations` for each provider account. This means it's possible 
to store a single key in many places.