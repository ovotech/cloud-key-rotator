# Cloud-Key-Rotator

This is a Golang program to assist with the reporting of Service Account key
ages, and rotating said keys once they pass a specific age threshold.

## Install

### From Binary Releases

Darwin, Linux and Windows Binaries can be downloaded from the [Releases](https://github.com/ovotech/cloud-key-rotator/releases) page.

Try it out:

```
$ cloud-key-rotator -h
```

### Docker image

An alpine based Docker image is available [here](https://hub.docker.com/r/ovotech/cloud-key-rotator).


## Getting Started

### Config

`cloud-key-rotator` picks up details about which key(s) to rotate, and locations
to update with new keys, from config.

Check out [examples](examples) for example config files. [Viper](https://github.com/spf13/viper)
is used as the Config framework, so config can be stored as JSON, TOML, YAML, or
HCL. To work, the file just needs to be called "config" (before whatever
extension you're using), and be present either in `/etc/cloud-key-rotator/` or
in the same directory the binary runs in.

### Authentication/Authorisation

You'll need to provide `cloud-key-rotator` with the means of authenticating into
any key provider that it'll be updating.

Authorisation is handled by the Default Credential Provider Chains for both
[GCP](https://cloud.google.com/docs/authentication/production#auth-cloud-implicit-go) and [AWS](https://docs.aws.amazon.com/sdk-for-java/v1/developer-guide/credentials.html#credentials-default).

### Mode Of Operation

`cloud-key-rotator` can operate in 2 different modes. Rotation mode, in which
keys are rotated, and non-rotation mode, which only posts the ages of the keys to
the Datadog metric API. You can specify which mode to operate in by using the
`RotationMode` boolean field in config.

### Age Thresholds

You can set the age threshold to whatever you want in the config, using the
`DefaultRotationAgeThresholdMins` field in config, or you can override on a
per-service-account-basis with the `RotationAgeThresholdMins` field. Key ages
are always measured in minutes.

`cloud-key-rotator` won't attempt to rotate a key until it's passed the age
threshold you've set (either default or the key-specific). This allows you to
run it as frequently as you want without worrying about keys being rotated too
much.

### Key Locations

Key locations is the term used for the places that keys are stored, that
ultimately get updated with new keys that are generated.

Currently, the following locations are supported:

- EnvVars CircleCI
- Secrets in GKE
- Files (encrypted via [mantle](https://github.com/ovotech/mantle) which
integrates with KMS) in GitHub

## Rotation Process

By design, the rotation process is sensitive; it attempts to verify its actions
as much as possible, and aborts immediately if it sees any errors. The idea
behind this approach is that it should be quick to re-run the tool (with new
keys being created) once issues have been resolved. Cloud Providers usually
limit the number of keys you can have attached to a Service Account at any one
time, so worth bearing this in mind when re-running manually after seeing errors.

The alternative to this, is for `cloud-key-rotator` to attempt to gracefully
handle errors, and for someone to manually patch things up afterwards. This
approach could lead to split-brain effect of key sources (places where the keys
are used) and confusion around what needs to be done before the old key can be
deleted. After all sources are using the new key, the same thing may happen on
the next key rotation, which doesn't lend itself well to automation.

Only the first key of a Service Account is handled by `cloud-key-rotator`. If it
handled more than one key, it could lead to complications when updating single
sources multiple times.

## Key Sources

The `AccountKeyLocations` section of config holds details of the places where the keys
are stored. E.g.:

```JSON
"AccountKeyLocations": [{
  "ServiceAccountName": "cloud-key-client-test",
  "RotationAgeThresholdMins": 60,
  "GitHub": {
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
deployment has been successful after committing to a GitHub repo. If that
verification isn't required, you can disable it using the `VerifyCircleCISuccess`
boolean.

For any GitHub key source that's configured, the whole process will be aborted
if there's no `KmsKey` value set. Unencrypted keys shouldn't ever be committed
to a Git repository.

## GPG Commit Signing

Commits to GitHub repositories are required to be GPG signed. In order to
achieve this, you just need to provide 4 things:

* `Username` of the GitHub user commits will be made on behalf of, set in config
* `Email` address of GitHub user, set in config
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

- Reduce keys to those of service accounts deemed to be valid (e.g. strip out
  user accounts if in rotation-mode)
- Filter keys to those deemed to be eligible (e.g. according to filtering rules
  configured by the user)
- For each eligible key:
 - Create new key
 - Update key locations
 - Verify update has worked (where possible)
 - Delete old key

## Contributions

Contributions are more than welcome. It should be straight forward plugging in
new integrations of new key providers and/or locations, so for that,
or anything else, please branch or fork and raise a PR.
