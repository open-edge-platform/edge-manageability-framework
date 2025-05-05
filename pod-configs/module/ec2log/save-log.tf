# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

locals {
  ssm_term_func     = "ssm-term"
  ssm_term_tpl      = "${local.ssm_term_func}.py.tpl"
  ssm_term_src      = "${local.ssm_term_func}.py"
  ssm_term_arch     = "${local.ssm_term_func}.zip"
  s3_bucket         = var.s3_prefix == "" ? "${var.cluster_name}-ec2log" : "${var.cluster_name}-${var.s3_prefix}-ec2log"
  ssm_doc_term_name = "orch-ec2log-${var.cluster_name}-term"
  lambda_term_name  = "orch-ec2log-${var.cluster_name}-term"
}

resource "aws_ssm_document" "push_log" {
  name            = local.ssm_doc_term_name
  document_format = "YAML"
  document_type   = "Command"

  content = <<DOC
schemaVersion: "2.2"
description: "Push logs to S3."
parameters:
  ASGNAME:
    type: "String"
    description: "Auto Scaling group name"
  LIFECYCLEHOOKNAME:
    type: "String"
    description: "LIFECYCLEHOOK name"
  BACKUPFILES:
    type: "String"
    description: "File to backup"
  SCRIPT:
    type: "String"
    description: "Script to generate additional logs"
mainSteps:
  -
    action: "aws:runShellScript"
    name: "pushlog"
    inputs:
      runCommand:
        - "export TOKEN=$(curl -X PUT 'http://169.254.169.254/latest/api/token' -H 'X-aws-ec2-metadata-token-ttl-seconds: 21600')"
        - "instanceid=$(curl -H \"X-aws-ec2-metadata-token: $TOKEN\" http://169.254.169.254/latest/meta-data/instance-id)"
        - "HOOKRESULT='CONTINUE'"
        - "region=$(curl -H \"X-aws-ec2-metadata-token: $TOKEN\" -s http://169.254.169.254/latest/meta-data/placement/availability-zone | sed 's/.$//')"
        - "tf=$(date '+%Y-%m-%d-%H-%M')"
        - "if [ -n \"{{SCRIPT}}\" ]; then bash -c \"{{SCRIPT}}\";fi"
        - "for f in {{BACKUPFILES}}; do"
        - "    if [ -f $f ]; then sudo /usr/bin/aws s3 cp $f s3://${local.s3_bucket}/${var.cluster_name}/$tf-$instanceid/$f; fi"
        - "done"
        - "/usr/bin/aws autoscaling complete-lifecycle-action --lifecycle-hook-name {{LIFECYCLEHOOKNAME}} --auto-scaling-group-name {{ASGNAME}} --lifecycle-action-result $HOOKRESULT --instance-id $instanceid --region $region"
DOC
  tags = {
    Cluster = var.cluster_name
  }
}

resource "local_file" "ssm_term_src" {
  content  = templatefile(
    "${path.module}/${local.ssm_term_tpl}", 
    {
      cluster_name     = var.cluster_name,
      upload_file_list = var.upload_file_list,
      script           = var.script
    }
  )
  filename = "${path.module}/${local.ssm_term_src}"
}

data "archive_file" "ssm_term_arch" {
  depends_on  = [local_file.ssm_term_src]
  type        = "zip"
  source_file = "${path.module}/${local.ssm_term_src}"
  output_path = "${path.module}/${local.ssm_term_arch}"
}

resource "aws_lambda_function" "ssm_term" {
  function_name    = local.lambda_term_name
  description      = "Save Orchestrator EKS node logs"
  role             = aws_iam_role.lambda.arn
  handler          = "${local.ssm_term_func}.lambda_handler"
  filename         = "${path.module}/${local.ssm_term_arch}"
  source_code_hash = data.archive_file.ssm_term_arch.output_base64sha256
  runtime          = "python3.9"
}

resource "aws_cloudwatch_log_group" "ssm_term" {
  name              = "/aws/lambda/${aws_lambda_function.ssm_term.function_name}"
  retention_in_days = var.cloudwatch_expire
  lifecycle {
    prevent_destroy = false
  }
}

resource "aws_lambda_permission" "allow_cloudwatch" {
  statement_id  = "AllowExecutionFromCloudWatch"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.ssm_term.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.instance_terminate.arn
}

resource "aws_s3_bucket" "logs" {
  bucket        = local.s3_bucket
  force_destroy = true

  lifecycle_rule {
    id      = "ec2log-terminate-retention"
    enabled = true
    expiration {
        days = var.s3_expire
    }
  }
}

resource "aws_iam_policy" "cloudwatch" {
  name        = "orch-ec2log-${var.cluster_name}-asg-lifecycle"
  path        = "/"
  description = "For EC2 and Lambda to log to cloudwatch"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "autoscaling:CompleteLifecycleAction",
        ]
        Effect   = "Allow"
        Resource = "*"
      },
      {
        Action = [
            "logs:PutLogEvents",
            "cloudwatch:PutMetricData",
        ],
        Effect   = "Allow",
        Resource = "*",
      },
    ]
  })
}

resource "aws_iam_policy" "s3" {
  name        = "orch-ec2log-${var.cluster_name}-s3"
  path        = "/"
  description = "For EC2 and Lambda to upload logs"

  policy = jsonencode({
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "AllowAuroraToExampleBucket",
            "Effect": "Allow",
            "Action": [
                "s3:PutObject",
                "s3:GetObject",
                "s3:AbortMultipartUpload",
                "s3:ListBucket",
                "s3:DeleteObject",
                "s3:GetObjectVersion",
                "s3:ListMultipartUploadParts"
            ],
            "Resource": [
                "${aws_s3_bucket.logs.arn}/*",
                "${aws_s3_bucket.logs.arn}"
            ]
        }
    ]
  })
}

resource "aws_iam_policy" "ssm" {
  name        = "orch-ec2log-${var.cluster_name}-ssm"
  path        = "/"
  description = "For Lambda function to execute SSM"

  policy = jsonencode({
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "AllowAuroraToExampleBucket",
            "Effect": "Allow",
            "Action": [
                "ssm:SendCommand",
                "ssm:GetCommandInvocation"
            ],
            "Resource": [
                "*"
            ]
        }
    ]
  })
}

data "aws_iam_role" "ec2_role" {
  name = var.nodegroup_role
}

resource "aws_iam_role_policy_attachment" "ec2_cloudwatch" {
  role       = data.aws_iam_role.ec2_role.name
  policy_arn = aws_iam_policy.cloudwatch.arn
}

resource "aws_iam_role_policy_attachment" "ec2_s3" {
  role       = data.aws_iam_role.ec2_role.name
  policy_arn = aws_iam_policy.s3.arn
}

resource "aws_iam_role" "lambda" {
  name = "orch-ec2log-${var.cluster_name}"

  assume_role_policy = jsonencode({
    "Version": "2012-10-17",
    "Statement": [
      {
        "Effect": "Allow",
        "Principal": {
          "Service": "lambda.amazonaws.com"
        },
        "Action": "sts:AssumeRole"
      }
    ]
  })

  tags = {
    tag-key = "orch-ec2log-lambda-${var.cluster_name}"
  }
}

resource "aws_iam_role_policy_attachment" "lambda_cloudwatch" {
  role       = aws_iam_role.lambda.name
  policy_arn = aws_iam_policy.cloudwatch.arn
}

resource "aws_iam_role_policy_attachment" "lambda_ssm" {
  role       = aws_iam_role.lambda.name
  policy_arn = aws_iam_policy.ssm.arn
}

data "aws_eks_node_group" "nodegroup1" {
  cluster_name    = var.cluster_name
  node_group_name = "nodegroup-${var.cluster_name}"
}

data "aws_eks_node_group" "observability" {
  cluster_name    = var.cluster_name
  node_group_name = "observability"
}

resource "aws_autoscaling_lifecycle_hook" "nodegroup1-terminating" {
  name                   = "${var.cluster_name}-nodegroup1-terminating"
  autoscaling_group_name = data.aws_eks_node_group.nodegroup1.resources[0].autoscaling_groups[0].name
  default_result         = "CONTINUE"
  heartbeat_timeout      = 2000
  lifecycle_transition   = "autoscaling:EC2_INSTANCE_TERMINATING"
}

resource "aws_autoscaling_lifecycle_hook" "observability-terminating" {
  name                   = "${var.cluster_name}-observability-terminating"
  autoscaling_group_name = data.aws_eks_node_group.observability.resources[0].autoscaling_groups[0].name
  default_result         = "CONTINUE"
  heartbeat_timeout      = 2000
  lifecycle_transition   = "autoscaling:EC2_INSTANCE_TERMINATING"
}

resource "aws_cloudwatch_event_rule" "instance_terminate" {
  name        = "orch-ec2log-term-${var.cluster_name}"
  description = "Send logs to S3 when ASG instances terminated"

  event_pattern = jsonencode({
    "source": ["aws.autoscaling"],
    "detail-type": ["EC2 Instance-terminate Lifecycle Action"],
    "detail": {
      "AutoScalingGroupName": [
        "${data.aws_eks_node_group.nodegroup1.resources[0].autoscaling_groups[0].name}",
        "${data.aws_eks_node_group.observability.resources[0].autoscaling_groups[0].name}"
      ],
      "LifecycleHookName": ["${var.cluster_name}-nodegroup1-terminating","${var.cluster_name}-observability-terminating"]
    }
  })
}

resource "aws_cloudwatch_event_target" "lambda_runssm_instance_terminate" {
  target_id = local.lambda_term_name
  rule      = aws_cloudwatch_event_rule.instance_terminate.name
  arn       = aws_lambda_function.ssm_term.arn
}