variable "ckr_resource_suffix" {
  default = ""
}

variable "ckr_schedule" {
  default = "0 10 * * 1-5"
}

variable "ckr_schedule_time_zone" {
  default = "Europe/London"
}

variable "deploying_accounts" {
  default = []
  type    = list(string)
}

variable "ckr_version" {}

variable "ckr_config" {}
