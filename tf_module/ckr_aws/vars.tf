variable "config_data" {
  default = ""
}

variable "ckr_version" {
}

variable "ckr_schedule" {
  default = "cron(0 10 ? * MON-FRI *)"
}

variable "enable_ssm_location" {
  type    = bool
  default = false
}

variable "ckr_trigger_description" {
  default = "Scheduled cloud key rotation"
}

variable "region" {
  type    = string
  default = "eu-west-1"
}

variable "ckr_timeout" {
  default = 300
  type    = number
}
