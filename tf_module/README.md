# Cloud Key Rotator: Terraform Module

A Terraform module hosted on the ovotech repository that creates an AWS Lambda to run Cloud Key Rotator.

## Usage - AWS

### Pre-requisite

You'll need a configuration json file which holds secrets and which keys to rotate.  [Examples](https://github.com/ovotech/cloud-key-rotator/tree/master/examples)
here and detail in the main
[README](https://github.com/ovotech/cloud-key-rotator/blob/master/README.md).

Either:

* Create a config.json. Store it in AWS Secrets Manager and
store the ARN in the `data` block below.

* Terraform the config.json using a template file and inject the secrets into the .json file as a variable - eg:

```
resource "aws_secretsmanager_secret_version" "ckr-config-json" {
  secret_id     = "${aws_secretsmanager_secret.ckr-config-secret.id}"
  secret_string = templatefile(
    "**<insert relative path to config.json template>**",
        {
      circle_api_key = var.circle_api_key # Sensitive API key injected to template
    }
        )
}
```

An example config.json template to rotate an SSM parameter that holds an IAM user's details and a CircleCi user (requiring a Circle API key to do so) is [here](https://github.com/ovotech/cloud-key-rotator/tree/master/examples/config-template.tmpl).

### Terraform usage

```
provider "aws" {
  version = "~> 2.28"
  region  = "eu-west-1"
}

data "aws_secretsmanager_secret_version" "ckr_secret" {
  secret_id = "**<Insert or reference your Secret ARN>**"
}

module "cloud-key-rotator" {
  source         = "terraform.ovotech.org.uk/pe/ckr/aws"
  config_file_path = data.aws_secretsmanager_secret_version.ckr_secret.secret_string

  version = "0.0.3"
  ckr_version = "0.27.18"
}
```

## Variables

* `version = "0.0.3"` -> The Terraform module version to use
* `ckr_version = "0.27.18"` -> The Cloud Key Rotator binary version to use
* (Optional) `ckr_schedule = "cron(0 10 ? * MON-FRI *)"` -> defaults to triggering 10am Monday-Friday
