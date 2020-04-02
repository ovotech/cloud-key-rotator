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
  version = "0.0.5"
  ckr_version = "0.27.18"
}
```

## Variables

* `version = "0.0.5"` -> The Terraform module version to use
* `ckr_version = "0.27.18"` -> The Cloud Key Rotator binary version to use
* (Optional) `ckr_schedule = "cron(0 10 ? * MON-FRI *)"` -> defaults to triggering 10am Monday-Friday
* (OptionaL) `config_data = <string>` -> Pass a json blob from any source containing your config file.