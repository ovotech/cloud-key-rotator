# Cloud Key Rotator: Terraform Module

Terraform modules hosted on the ovotech repository that creates serverless
functions and their associated resources in order to run `cloud-key-rotator` in
either AWS or GCP.

## Usage - AWS

### Pre-requisite

You'll need a configuration json file which holds secrets and which keys to rotate. [Examples](https://github.com/ovotech/cloud-key-rotator/tree/master/examples)
here and detail in the main
[README](https://github.com/ovotech/cloud-key-rotator/blob/master/README.md).

Either:

- Create a config.json. Apply the Terraform module, and then replace the `placeholder` ckr-config secret value with your json blob.

- Create a config.json. Store it somewhere (in AWS or otherwise), and inject it into Terraform to the `config_data` variable as a string.

An example config.json template to rotate an SSM parameter that holds an IAM user's details and a CircleCi user (requiring a Circle API key to do so) is [here](https://github.com/ovotech/cloud-key-rotator/tree/master/examples/config-template.tmpl).

### Terraform usage

```hcl
provider "aws" {
  version = "~> 2.28"
  region  = "eu-west-1"
}


module "cloud-key-rotator" {
  source      = "terraform.ovotech.org.uk/pe/ckr/aws"
  version     = "1.1.1"
  ckr_version = "0.27.51"
}
```

### Variables

- `version = "1.1.1"` -> The Terraform module version to use.
- `ckr_version = "0.27.51"` -> The Cloud Key Rotator binary version to use.
- (Optional) `ckr_schedule = "0 10 * * 1-5"` -> Defaults to triggering 10am Monday-Friday.
- (Optional) `config_data = <string>` -> Pass a json blob from any source containing your config file.
- (Optional) `enable_ssm_location = false` -> Whether to create an IAM policy allowing `ssm:PutParameter`.
  Set this to `true` if using SSM as a `cloud-key-rotator` location.
- (Optional) `region = <string>` -> pass aws region. Defaults to `eu-west-1` if not set.

## Usage - GCP

This module creates a Cloud Function to run the Cloud Key Rotator and a job in Cloud Scheduler to run the Cloud Function.

You will need the following APIs enabled in your project:

- cloudbuild.googleapis.com
- cloudscheduler.googleapis.com
- cloudfunctions.googleapis.com
- appengine.googleapis.com

Unfortunately GCP Cloud Scheduler requires an AppEngine App to be present in
the project before jobs can be created. Currently this must be done outside
of the module, though there are plans to bring it inside the module in future.

The module supports Terraform version 0.12.6 and up.

### Terraform usage

```hcl
provider "google" {
  version = "~> 3.22.0"
  region  = "europe-west1"
  project = "your-project"
}

module "cloud-key-rotator" {
  source = "terraform.ovotech.org.uk/pe/ckr/gcp"
  version = "1.0.0"
  project = "your-project"
  ckr_version = "0.27.43"
  ckr_resource_suffix = "my-project-name"
  ckr_config = <<EOF
{
  "EnableKeyAgeLogging": true,
  "RotationMode": false,
  "CloudProviders": [{
    "Project":"${var.project_name}",
    "Name": "gcp"
  }],
}
EOF
}

```

### Variables

- `version = "1.0.0"` -> The Terraform module version to use.
- `project = <string>` -> The project ID of the target project. This is not inferred from the provider.
- `ckr_version = "0.27.43"` -> The Cloud Key Rotator binary version to use.
- `ckr_config = <string>` -> Pass a json blob from any source containing your config file.
- (Optional) `ckr_resource_suffix = "my-project-name"` -> Will be appended to the bucket, cloud function, custom role. Defaults to a 3 character random string
  service account and scheduler job names to prevent naming conflicts
- (Optional) `ckr_schedule = "0 10 * * 1-5"` -> Defaults to triggering 10am Monday-Friday.
- (Optional) `ckr_schedule_time_zone = "Europe/London"` -> The time zone for the scheduler job. Defaults to Europe/London
- (Optional) `deploying_accounts = ["serviceAccount:terraform@myproject.iam.gserviceaccount.com"]` -> Any accounts which
  will be deploying the CKR terraform but do not have the iam.serviceAccountUser permission for the whole project. This
  gives the supplied accounts iam.serviceAccountUser permissions for the Cloud Key Rotator service account which is
  necessary to deploy the terraform module. Defaults to an empty list
- (Optional) `ckr_timeout = 300` -> Timeout (in seconds) for the function. Default value is 300 seconds. Cannot be more than 540 seconds.
