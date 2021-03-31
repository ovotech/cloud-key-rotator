terraform {
  backend "gcs" {
    bucket = "prod-eng-dev-terraform"
    prefix = "ckr_e2e/"
  }
}

provider "google" {
  project = "pe-dev-185509"
  region  = "europe-west3"
}

module "cloud-key-rotator" {
  source      = "../ckr_gcp"
  ckr_version = "0.27.34"
  ckr_config  = <<EOF
  {
    "EnableKeyAgeLogging": true,
    "RotationMode": false
  }
  EOF
}
