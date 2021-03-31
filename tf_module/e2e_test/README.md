# Terraform Module E2E Test

This directory aims to provide a quick way to validate terraform module changes. The
terraform in the e2e test creates the gcp cloud key rotator module with the minimum allowed
inputs on the oldest permitted terraform version (0.12).

Please use `terraform destroy` when you're done to remove the resources created during testing.
