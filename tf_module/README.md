# Cloud Key Rotator: Terraform Module

A Terraform module hosted on the ovotech repository that creates an AWS Lambda to run Cloud Key Rotator.

## Usage - AWS

### Pre-requisite

You'll need a configuration json file which holds secrets and which keys to rotate.  [Examples](https://github.com/ovotech/cloud-key-rotator/tree/master/examples)
here and detail in the main
[README](https://github.com/ovotech/cloud-key-rotator/blob/master/README.md).

Either:

* Create a config.json. Apply the Terraform module, and then replace the `placeholder` ckr-config secret value with your json blob.

* Create a config.json. Store it somewhere (in AWS or otherwise), and inject it into Terraform to the `config_data` variable as a string.

An example config.json template to rotate an SSM parameter that holds an IAM user's details and a CircleCi user (requiring a Circle API key to do so) is [here](https://github.com/ovotech/cloud-key-rotator/tree/master/examples/config-template.tmpl).

### Terraform usage

```
provider "aws" {
  version = "~> 2.28"
  region  = "eu-west-1"
}


module "cloud-key-rotator" {
  source         = "terraform.ovotech.org.uk/pe/ckr/aws"
  version = "0.1.0"
  ckr_version = "0.27.28"
}
```

### Variables

* `version = "0.1.0"` -> The Terraform module version to use.
* `ckr_version = "0.27.28"` -> The Cloud Key Rotator binary version to use.
* (Optional) `ckr_schedule = "cron(0 10 ? * MON-FRI *)"` -> Defaults to triggering 10am Monday-Friday.
* (Optional) `config_data = <string>` -> Pass a json blob from any source containing your config file.
* (Optional) `enable_ssm_location = false` -> Whether to create an IAM policy allowing `ssm:PutParameter`.
Set this to `true` if using SSM as a `cloud-key-rotator` location.

## Usage - GCP

This module creates a Cloud Function to run the Cloud Key Rotator and a job in Cloud Scheduler to run the Cloud Function.

You will need the following APIs enabled in your project:

* cloudbuild.googleapis.com
* cloudscheduler.googleapis.com
* cloudfunctions.googleapis.com
* appengine.googleapis.com

### Terraform usage

```
provider "google" {
  version = "~> 3.22.0"
  region  = "europe-west1"
  project = "your-project"
}

module "cloud-key-rotator" {
  source = "terraform.ovotech.org.uk/pe/ckr/gcp"
  version = "0.0.1"
  ckr_version = "0.27.28"
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

* `version = "0.1.0"` -> The Terraform module version to use.
* `ckr_version = "0.27.28"` -> The Cloud Key Rotator binary version to use.
* `ckr_config = <string>` -> Pass a json blob from any source containing your config file.
* `ckr_resource_suffix = "my-project-name"` -> Will be appended to the bucket, cloud function, custom role
  service account and scheduler job names to prevent naming conflicts
* (Optional) `ckr_schedule = "0 10 * * 1-5"` -> Defaults to triggering 10am Monday-Friday.
* (Optional) `ckr_schedule_time_zone = "Europe/London"` -> The time zone for the scheduler job. Defaults to Europe/London
