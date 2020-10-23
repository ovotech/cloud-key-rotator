
data "aws_caller_identity" "current" {}

locals {
  account_id = data.aws_caller_identity.current.account_id
}

# IAM

resource "aws_iam_role" "cloudkeyrotator_role" {
  name = "CloudKeyRotatorRole"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "aws_iam_policy" "ckr_log_policy" {
  name = "CloudKeyRotatorLogPolicy"
  path = "/"

  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "logs:CreateLogStream",
                "logs:PutLogEvents"
            ],
            "Resource": [
                "arn:aws:logs:eu-west-1:${local.account_id}:log-stream:*:*:*",
                "arn:aws:logs:eu-west-1:${local.account_id}:log-group:/aws/lambda/cloud-key-*"
            ]
        },
        {
            "Effect": "Allow",
            "Action": "logs:CreateLogGroup",
            "Resource": "arn:aws:logs:eu-west-1:${local.account_id}:*"
        }
    ]
}
EOF
}

# SSM is a valid location of the cloud-key-rotator, so allow ssm:PutParameter
# if enabled
resource "aws_iam_policy" "ckr_ssm_policy" {
  count = var.enable_ssm_location ? 1 : 0
  name = "CloudKeyRotatorSsmPolicy"
  path = "/"

  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ssm:PutParameter"
            ],
            "Resource": [
                "arn:aws:ssm:eu-west-1:${local.account_id}:parameter/*"
            ]
        }
    ]
}
EOF
}

resource "aws_iam_policy" "ckr_policy" {
    name   = "CloudKeyRotatorPolicy"
    path   = "/"
    policy = jsonencode(
        {
            Statement = [
                {
                    Action   = [
                        "iam:DeleteAccessKey",
                        "iam:CreateAccessKey",
                        "iam:ListAccessKeys",
                    ]
                    Effect   = "Allow"
                    Resource = [
                        "arn:aws:iam::*:user/*",
                    ]
                },
                {
                    Action   = "iam:ListUsers"
                    Effect   = "Allow"
                    Resource = "arn:aws:iam::*:*"
                },
                {
                    Action   = "secretsmanager:GetSecretValue"
                    Effect   = "Allow"
                    Resource = [
                        aws_secretsmanager_secret.ckr-config.arn,
                    ]
                },
            ]
            Version   = "2012-10-17"
        }
    )
}

resource "aws_iam_role_policy_attachment" "attach-ckr-log-policy" {
  role       = aws_iam_role.cloudkeyrotator_role.name
  policy_arn = aws_iam_policy.ckr_log_policy.arn
}

resource "aws_iam_role_policy_attachment" "attach-ckr-policy" {
  role       = aws_iam_role.cloudkeyrotator_role.name
  policy_arn = aws_iam_policy.ckr_policy.arn
}

# only create ssm attachment if SSM is enabled (indicating it's being used
# as a cloud-key-rotator location)
resource "aws_iam_role_policy_attachment" "attach-ckr-ssm-policy" {
  count = var.enable_ssm_location ? 1 : 0
  role       = aws_iam_role.cloudkeyrotator_role.name
  policy_arn = aws_iam_policy.ckr_ssm_policy[0].arn
}


resource "aws_lambda_function" "cloud_key_rotator" {
  description   = "A function for rotating cloud keys"
  s3_bucket     = "ckr-terraform-module-code"
  s3_key        = "cloud-key-rotator_${var.ckr_version}_lambda.zip"
  function_name = "cloud-key-rotator"
  role          = aws_iam_role.cloudkeyrotator_role.arn
  handler       = "cloud-key-rotator-lambda"
  timeout       = 300
  runtime       = "go1.x"
}

resource "aws_cloudwatch_event_rule" "cloud-key-rotator-trigger" {
  name                = "cloud-key-rotator-trigger"
  description         = "Daily at 10am"
  schedule_expression = var.ckr_schedule
}

resource "aws_cloudwatch_event_target" "check_every_five_minutes" {
  rule      = aws_cloudwatch_event_rule.cloud-key-rotator-trigger.name
  target_id = "cloud_key_rotator"
  arn       = aws_lambda_function.cloud_key_rotator.arn
}

resource "aws_lambda_permission" "allow_cloudwatch_to_call_lambda" {
  statement_id  = "AllowExecutionFromCloudWatch"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.cloud_key_rotator.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.cloud-key-rotator-trigger.arn
}

# Secret and config file

resource "aws_secretsmanager_secret" "ckr-config" {
  # Create a correctly named secret
  name        = "ckr-config"
  description = "A JSON file that configures Cloud Key Rotator"
}

resource "aws_secretsmanager_secret_version" "placeholder_config" {
  count = var.config_data == "" ? 1 : 0
  # If config_data is unset (or false), create placeholder secret
  secret_id     = aws_secretsmanager_secret.ckr-config.id
  secret_string = "placeholder"

  lifecycle {
    ignore_changes = [
      secret_string
    ]
  }
}

resource "aws_secretsmanager_secret_version" "ckr-config-string" {
  count = var.config_data != "" ? 1 : 0
  # If config_data is set, use as secret string
  secret_id     = aws_secretsmanager_secret.ckr-config.id
  secret_string = var.config_data
}