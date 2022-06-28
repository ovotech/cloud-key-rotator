# GitHub Secrets Example

## Pre-requisites

In order to rotate a key that's stored in GitHub secrets, you'll need:

1. A GitHub personal access token of a user that has permission to update
secrets in the target repo
2. Auth to actually perform the rotation operation with whichever cloud provider
you're using. This will require a service-account or user (with the
cloud-provider you're rotating with) that has the required set of permissions.
Then, auth will need to be given to `cloud-key-rotator` (usually in the form of
a .json file or env vars).

## Configuration

For updating a secret in a single repo:

```json
  "AccountKeyLocations": [
    {
      "ServiceAccountName": "my_aws_machine_user",
      "GitHub": [
        {
          "Owner": "my_org",
          "Repo": "my_repo",

        },
        {
          "Owner": "my_org",
          "Repo": "my_repo",

        },
      ]
    }
  ],
  "Credentials": {
    "GitHubAPIToken": "my_github_api_token"
  }
```

For updating a secret in multiple repos:

```json
  "AccountKeyLocations": [
    {
      "ServiceAccountName": "my_aws_machine_user",
      "GitHub": [
        {
          "Owner": "my_org",
          "Repo": "my_repo",

        },
        {
          "Owner": "my_org",
          "Repo": "my_repo",

        },
      ]
    }
  ],
  "Credentials": {
    "GitHubAPIToken": "my_github_api_token"
  }
```

When rotating AWS keys, there are some optional fields,
`keyIDEnvVar` and `keyEnvVar`, that represent the env var names in GitHub,
defaulting to values `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`
respectively.

So, if you store your Key ID and Key values in env vars in GitHub that're
named differently, you could set something like this instead:

```json
    "GitHub": [{
      "Owner": "my_org",
      "Repo": "my_repo",
      "KeyIDEnvVar": "AWS_KEY_ID",
      "KeyEnvVar": "AWS_KEY"
    }]
```

When rotating GCP keys, to override the default GitHub env var name
(`GCLOUD_SERVICE_KEY`), you only need to override the `KeyEnvVar` value (as only
a single value, the key, is needed for GCP)

```json
    "GitHub": [{
      "Owner": "my_org",
      "Repo": "my_repo",
      "KeyEnvVar": "GCP_KEY"
    }]
```

If you use a GCP key to interact with GCR, especially for downloading
docker images for GitHub to run your pipelines, then you may want the
key to be stored as JSON directly, rather than base64 encoded (which is
the default). You can do this easily by setting the `Base64Decode` option
to true in the GitHub location block, like so:

```json
    "GitHub": [{
      "Owner": "my_org",
      "Repo": "my_repo",
      "Base64Decode": true
    }]
```