# cloud-key-rotator

This is a Golang program to assist with the reporting of Service Account key
ages, and rotating said keys once they pass a specific age threshold.

You can set the age threshold to whatever you want in the config, using the
`RotationAgeThresholdMins` field (the value is always in minutes).

`cloud-key-rotator` can operate in 2 different modes. Rotation mode, in which
keys are rotated, and non-rotation mode, which only posts the ages of the keys to
the Datadog metric API. You can specify which mode to operate in by using the
`RotationMode` boolean field in config.

Check out [config.json](config.json) for an example config file. [Viper](https://github.com/spf13/viper)
is used as a Config solution, so config can be stored as JSON, TOML, YAML, or
HCL. To work, the file just needs to be called "config" (before whatever
extension you're using), and be present in the `/etc/cloud-key-rotator/`
directory.

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

The `KeySources` section of config holds details of the places where the keys
are stored. E.g.:

```JSON
"KeySources": [{
  "ServiceAccountName": "cloud-key-rotator-test",
  "GitHub": {
    "FilePath": "service-account.txt",
    "OrgRepo": "myorg/myrepo",
    "VerifyCircleCISuccess": true,
    "CircleCIDeployJobName": "dummy_deploy_with_wait"
  },
  "CircleCI": [{
    "UsernameProject" : "ovotech/cloud-key-rotator-poc",
    "EnvVar" : "GCLOUD"
  }]
}],
```

`cloud-key-rotator` has integrations into GitHub and CircleCI, which allows it
not only to update those sources with the new key, but also to verify that a
deployment has been successful after committing to a GitHub repo. If that
verification isn't required, you can disable it using the `VerifyCircleCISuccess`
boolean.

Only a single `GitHub` element is permitted, as you shouldn't be holding your
keys in multiple repositories, whereas `CircleCI` is an array as it's much more
likely that you'll be storing the same key in multiple environment variables.

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

## Include/Exclude Service Accounts

You may want to only include or exclude specific Service Accounts. This is
possible using the `IncludeSAs` and `ExcludeSAs` config fields respectively.

E.g.:

```JSON
"ExcludeSAs": [],
"IncludeSAs": ["cloud-key-rotator-test"],
```

`IncludeSAs` overrides `ExcludeSAs` entirely, meaning that if you want to
Exclude, you'll have to specify an empty array for the Include field.

If any Service Accounts are specified in `IncludeSAs`, then keys for any other
Service Accounts will be completely filtered out.

Notice how the name of the Service Account is used. In the case of GCP, this is
everything preceding the `@[project].iam.gserviceaccount.com` string in the
Service Account's email address.

## Rotation Flow

1. Filter keys to those deemed to be eligible
2. For each eligible key:
 1. Create new key
 2. Update key sources
 3.  Delete old key

## Contributions

Contributions are more than welcome. It should be straight forward plugging in
new integrations of new key sources, so for that, or anything else, please
branch or fork and raise a PR.
