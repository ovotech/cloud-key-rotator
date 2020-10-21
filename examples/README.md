# Getting Started

## Key Dater

The easiest way to get started with `cloud-key-rotator` is to use it solely to
obtain ages of your service account keys.

### Storing Config

If running in AWS Lambda, store the config in SecretManager in a secret named
`ckr-config`.

If running in GCP CloudFunctions, store the config in GCS, e.g. in
`your_bucket/ckr-config.json`, then set an env var 
`CKR_BUCKET_NAME=your_bucket`.

Otherwise, when running the binary or code directly, pop the config into
a file called `config.json` in `/etc/cloud-key-rotator/`.

### Config Contents

For scraping of AWS key ages:

```json

{
  "EnableKeyAgeLogging": true,
  "RotationMode": false,
  "CloudProviders": [{
    "Name":"aws"
  }]
}
```

For scraping of GCP key ages:

```json

{
  "EnableKeyAgeLogging": true,
  "RotationMode": false,
  "CloudProviders": [{
    "Project":"my-project",
    "Name": "gcp"
  }]
}
```

This will log the age of each key to stdout.

If you add a Datadog struct to the config, you can get `cloud-key-rotator` to post metrics to Datadog, too:

```json
  "Datadog": {
    "MetricEnv": "prod",
    "MetricName": "cloud-key-rotator.age",
    "MetricProject": "my_project",
    "MetricTeam": "my_team"
  },
  "DatadogAPIKey": "okj23434poz3j4o324p455oz3j4o324",
```

### Authentication

Regardless of where you run the `cloud-key-rotator` application, you'll need
to provide it with the means to authenticate into AWS or GCP, to allow it to
scrape the key ages.

This is where running in Lambda or CloudFunctions can really pay off, as you
can assign a Role or ServiceAccount (respectively), to permit the required
IAM read access.

### Examples

If you're after config examples for how to rotate keys, go to [AWS](./aws) or [GCP](./gcp).

If you want a more detailed example for storing the keys in a specific location
(e.g. CircleCI or GCS) then take a look [here](./locations).
