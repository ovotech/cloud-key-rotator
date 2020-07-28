# Cloud-Key-Rotator
[![CircleCI](https://circleci.com/gh/ovotech/cloud-key-rotator/tree/master.svg?style=svg)](https://circleci.com/gh/ovotech/cloud-key-rotator/tree/master)

This is a Golang program to assist with the reporting of Service Account key
ages, and rotating said keys once they pass a specific age threshold.

The tool can update keys held in the following locations:

* Atlas (mongoDB)
* CircleCI env vars
* CircleCI contexts
* GCS
* Git
* GoCd
* K8S (GKE only)
* SSM (AWS Parameter Store)

The tool is packaged as an executable file for native invocation, and as a zip
 file for deployment as an AWS Lambda.

## Install

### From Binary Releases

Darwin, Linux and Windows Binaries can be downloaded from the
 [Releases](https://github.com/ovotech/cloud-key-rotator/releases) page.

Try it out:

```
$ cloud-key-rotator -h
```

### Docker Image

An Alpine-based Docker image is available [here](https://hub.docker.com/r/ovotech/cloud-key-rotator).

## Getting Started

### Config

`cloud-key-rotator` picks up details about which key(s) to rotate, and locations
to update with new keys, from config.

Check out [examples](examples) for example config files. [Viper](https://github.com/spf13/viper)
is used as the config framework, so config can be stored as JSON, TOML, YAML or
HCL.

For native invocation, the file needs to be called "config" (before whatever
extension you're using), and be present either in `/etc/cloud-key-rotator/` or
in the same directory the binary runs in.  For AWS Lambda invocation, the config needs
to be set as a plaintext secret in the AWS Secrets Manager, using a default key name
 of "ckr-config".

### Authentication/Authorisation

You'll need to provide `cloud-key-rotator` with the means of authenticating into
any key provider that it'll be updating.

Authorisation is handled by the Default Credential Provider Chains for both
[GCP](https://cloud.google.com/docs/authentication/production#auth-cloud-implicit-go) and
 [AWS](https://docs.aws.amazon.com/sdk-for-java/v1/developer-guide/credentials.html#credentials-default).

### Mode Of Operation

`cloud-key-rotator` can operate in two different modes:

1. Rotation mode - in which keys are rotated; and
2. Non-rotation mode - which only posts the ages of keys to the Datadog Metric API.

The boolean field `RotationMode` config controls the mode of operation.

### Age Thresholds

You can set the age threshold to whatever you want in the config, using the
`DefaultRotationAgeThresholdMins` field in config, or you can override on a
per-service-account-basis with the `RotationAgeThresholdMins` field. Key ages
are always measured in minutes.

`cloud-key-rotator` will not attempt to rotate a key until it's passed the age
threshold you've set (either default or the key-specific). This allows you to
run the tool as frequently as you want without worrying about keys being rotated
excessively.

### Key Locations

"Key locations" is the term used for the places where keys are stored, which will
ultimately be updated with the new keys that are generated.

Currently, the following locations are supported:

* Atlas (mongoDB)
* CircleCI env vars
* CircleCI contexts
* GCS
* Git (files encrypted with [mantle](https://github.com/ovotech/mantle) which
integrates with KMS))
* GoCd
* K8S (GKE only)
* SSM (AWS Parameter Store)

## Rotation Process

The tool attempts to verify its actions as much as possible and aborts
immediately if it encounters an error.  By design, the tool does **not** attempt to
handle errors gracefully and continue, since this can lead to a "split-brain effect",
with keys out-of-sync in various locations.

It should be quick to re-run the tool (with new keys being created) once issues
 have been resolved.   Note that cloud providers usually limit the number of
 keys you can have attached to a Service Account at any one time, so it is
 worth bearing this in mind when re-running manually after seeing errors.

Only the first key of a Service Account is handled by `cloud-key-rotator`. If
it handled more than one key, it could lead to complications when updating
single sources multiple times.

## Key Sources

The `AccountKeyLocations` section of config holds details of the places where the keys
are stored, e.g.:

```JSON
"AccountKeyLocations": [{
  "ServiceAccountName": "cloud-key-client-test",
  "RotationAgeThresholdMins": 60,
  "Git": {
    "FilePath": "service-account.txt",
    "OrgRepo": "ovotech/cloud-key-rotator",
    "VerifyCircleCISuccess": true,
    "CircleCIDeployJobName": "dummy_deploy_with_wait"
  },
  "CircleCI": [{
    "UsernameProject": "ovotech/cloud-key-rotator",
    "KeyEnvVar": "ENV_VAR_NAME"
  }],
  "K8s": [{
    "Project": "my_project",
    "Location": "europe-west2-b",
    "ClusterName": "cluster_name",
    "Namespace": "uat",
    "SecretName": "key-rotate-test-secret",
    "DataName": "my-key.json"
  }]
}]
```

`cloud-key-rotator` has integrations into GitHub and CircleCI, which allows it
not only to update those sources with the new key, but also to verify that a
deployment has been successful after committing to a GitHub repository. If that
verification isn't required, you can disable it using the `VerifyCircleCISuccess`
boolean.

For any Git key location, the whole process will be aborted
if there is no `KmsKey` value set. Unencrypted keys should **never** be committed
to a Git repository.

## GPG Commit Signing

Commits to Git repositories are required to be GPG signed. In order to
achieve this, you need to provide 4 things:

* `Username` of the Git user commits will be made on behalf of, set in config
* `Email` address of Git user, set in config
* `ArmouredKeyRing`, aka GPG private key, stored in `/etc/cloud-key-rotator/akr.asc`
* `Passphrase` to the ArmouredKeyRing

e.g. along with the `akr.asc` file, you should set the following:
```JSON
"AkrPass": "change_me",
"GitName": "git-name",
"GitEmail": "change_me@example.com",
```

## Filtering Service Accounts

You may want to only include or exclude specific Service Accounts. This is
possible using `AccountFilter`.

E.g.:

```JSON
"AccountFilter": {
  "Mode": "include",
  "Accounts": [{
    "Provider": {
      "Name": "gcp",
      "Project": "my-project"
    },
    "ProviderAccounts": [
      "cloud-key-client-test"
    ]
  }]
}
```

`Mode` field is either `include` or `exclude`. If you omit the `AccountFilter`,
the rotation process will fail.

Notice how the name of the Service Account is used. In the case of GCP, this is
everything preceding the `@[project].iam.gserviceaccount.com` string in the
Service Account's email address.

## Rotation Flow

1. Reduce keys to those of service accounts deemed to be valid (e.g. strip out
  user accounts if in rotation-mode)
2. Filter keys to those deemed to be eligible (e.g. according to filtering rules
  configured by the user)
3. For each eligible key:

  * Create new key
  * Update key locations
  * Verify update has worked (where possible)
  * Delete old key

## Contributions

Contributions are more than welcome. It should be straightforward plugging in
integrations of new key providers and/or locations, so for that,
or anything else, please branch or fork and raise a PR.
