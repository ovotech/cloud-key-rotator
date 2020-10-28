locals {
  key_rotator_filename = "cloud-key-rotator-${var.ckr_version}.zip"
}

resource "google_service_account" "key_rotator_service_account" {
  account_id   = "ckr-${var.ckr_resource_suffix}"
  display_name = "Service account which runs the cloud key rotation cloud function"
}

data "google_client_config" "current_provider" {}

resource "google_storage_bucket" "key_rotator_bucket" {
  name     = "ckr-${var.ckr_resource_suffix}"
  location = data.google_client_config.current_provider.region
}

data "external" "key_rotator_zip" {
  program = ["${path.module}/download-zip.sh", var.ckr_version, local.key_rotator_filename]
}

resource "google_storage_object_access_control" "key_rotator_config_access" {
  object   = google_storage_bucket_object.key_rotator_cloud_function_config.output_name
  bucket   = google_storage_bucket.key_rotator_bucket.name
  role     = "READER"
  entity   = "user-${google_service_account.key_rotator_service_account.email}"
}

resource "google_storage_bucket_object" "key_rotator_cloud_function_zip" {
  name     = local.key_rotator_filename
  bucket   = google_storage_bucket.key_rotator_bucket.name
  source   = data.external.key_rotator_zip.result.output_filename
}

resource "google_cloudfunctions_function" "key_rotator_cloud_function" {
  name        = "ckr-${var.ckr_resource_suffix}"
  description = "This is a cloud function for rotating service account keys"
  runtime     = "go113"

  available_memory_mb   = 128
  source_archive_bucket = google_storage_bucket.key_rotator_bucket.name
  source_archive_object = google_storage_bucket_object.key_rotator_cloud_function_zip.name
  trigger_http          = true
  entry_point           = "Request"
  service_account_email = google_service_account.key_rotator_service_account.email

  environment_variables = {
    CKR_BUCKET_NAME = google_storage_bucket.key_rotator_bucket.name
  }

  // Cloud functions can take a long time to change
  timeouts {
    create = "10m"
    update = "10m"
    delete = "10m"
  }
}

resource "google_cloudfunctions_function_iam_member" "key_rotator_invoker_perms" {
  project        = google_cloudfunctions_function.key_rotator_cloud_function.project
  region         = google_cloudfunctions_function.key_rotator_cloud_function.region
  cloud_function = google_cloudfunctions_function.key_rotator_cloud_function.name

  role   = "roles/cloudfunctions.invoker"
  member = "serviceAccount:${google_service_account.key_rotator_service_account.email}"
}

resource "google_project_iam_member" "key_rotator_cloud_run_perms" {
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.key_rotator_service_account.email}"
}

resource "google_project_iam_custom_role" "key_rotator_custom_role" {
  role_id     = "cloudKeyRotator_${replace(var.ckr_resource_suffix, "-", "_")}"
  title       = "Custom role for the cloud key rotator"
  description = "This role gives the permissions necessary to rotate the cloud keys using the cloud key rotator tool"
  permissions = [
    "iam.serviceAccounts.list",
    "iam.serviceAccountKeys.list",
    "iam.serviceAccountKeys.create",
    "iam.serviceAccountKeys.delete"
  ]
}

resource "google_project_iam_member" "key_rotator_custom_perms" {
  role     = google_project_iam_custom_role.key_rotator_custom_role.id
  member   = "serviceAccount:${google_service_account.key_rotator_service_account.email}"
}

resource "google_cloud_scheduler_job" "key_rotator_scheduled_job" {
  name             = "ckr-${var.ckr_resource_suffix}"
  description      = "Job to routinely rotate service account keys"
  schedule         = var.ckr_schedule
  time_zone        = var.ckr_schedule_time_zone
  attempt_deadline = "320s"

  http_target {
    http_method = "GET"
    uri         = google_cloudfunctions_function.key_rotator_cloud_function.https_trigger_url

    oidc_token {
      service_account_email = google_service_account.key_rotator_service_account.email
    }
  }
}

resource "google_storage_bucket_object" "key_rotator_cloud_function_config" {
  name     = "ckr-config.json"
  bucket   = google_storage_bucket.key_rotator_bucket.name
  content  = var.ckr_config
}
