variable "ckr_version" {
  type        = string
  description = "The Cloud Key Rotator binary version to use."
}

variable "ckr_config" {
  type        = string
  description = "The JSON configuration for the Cloud Key Rotator to use"
}

variable "ckr_resource_suffix" {
  default     = ""
  type        = string
  description = "Suffix to append to resources to avoid naming conflicts if multiple cloud key rotator modules are in use in your project"
}

variable "ckr_schedule" {
  default     = "0 10 * * 1-5"
  type        = string
  description = "Cron style schedule at which to trigger the Cloud Key rotator"
}

variable "ckr_schedule_time_zone" {
  default     = "Europe/London"
  type        = string
  description = "The time zone for the scheduler job to schedule in"
}

variable "deploying_accounts" {
  default     = []
  type        = list(string)
  description = "List of accounts which will be deploying the CKR terraform. This needs to be given if you are not giving the deploying accounts the iam.serviceAccountUser permission for the whole project"
}

variable "ckr_timeout" {
  default     = 300
  type        = number
  description = "(Optional) Timeout (in seconds) for the function. Default value is 300 seconds. Cannot be more than 540 seconds."
}
