variable "config_data" {
  default = ""
}

variable "ckr_version" {
}

variable "ckr_schedule" {
  default = "cron(0 10 ? * MON-FRI *)"
}

variable "enable_ssm_location" {
  type = bool
  default = false
}