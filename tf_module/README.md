# Cloud Key Rotator: Terraform Module

A Terraform module hosted on the ovotech repository.

## Usage - AWS

Pre-requisite: Create a config.json. Store it in AWS Secrets Manager and
store the ARN in the `data` block below. # TODO expand on this

```
provider "aws" {
  version = "~> 2.28"
  region  = "eu-west-1"
}

data "aws_secretsmanager_secret_version" "ckr_secret" {
  secret_id = "arn:aws:secretsmanager:eu-west-1:454487269581:secret:ckr_test-kZoKjT"
}

module "cloud-key-rotator" {
  source         = "terraform.ovotech.org.uk/pe/ckr/aws"
  config_file_path = data.aws_secretsmanager_secret_version.ckr_secret.secret_string
  version = "0.0.3"
  ckr_version = "0.27.18"
}
```

## Variables

`version = "0.0.3"` -> The Terraform module version to use
`ckr_version = "0.27.18"` -> The Cloud Key Rotator binary version to use
(Optional) `ckr_schedule = "cron(0 10 ? * MON-FRI *)"` -> defaults to triggering 10am Monday-Friday
