# Save EKS node logs on S3 when an node is terminated

This module is an option in the cluster module if the variable *enable_ec2log*
is set. It push the logs in the EKS node EC2 instance to an S3 before the
instance is terminated for troubleshooting purposes.

## Background

Sometimes when an EKS cluster experiences critical issues, it terminates the
nodes in trouble and restarts new ones. The logs in the terminated nodes will be
removed as the result and will not been available for troubleshooting. We need a
way to keep the logs.

## How it works

The following main AWS resources are created for each cluster and involved in
the procedure:

- An Auto Scaling Group (ASG) lifecycle hook for each node group that triggers
  when instances are terminating.

- A CloudWatch Event Rule that captures the termination event and invokes the
  Lambda function.

- A Lambda function which responds to the terminating event. It passes the
  parameters and executes the SSM document.

- An SSM document which uploads the logs to the S3 bucket.

- An S3 bucket to save the logs with appropriate lifecycle rules for log
  retention.

- IAM roles and policies for the Lambda function and EC2 instances to interact
  with needed services.

## Logs included in uploads to S3 by default

- /var/log/messages*

- /var/log/aws-routed-eni/*

- /var/log/dmesg

- Output of *journalctl -xeu kubelet* command.

- Output of *free* command.

- Output of *df* command.

- Output of *top* command.

Other logs can be added to the upload list by setting the variables when calling
the module.

## Variables

Set the following variables in the cluster variable file to enable collecting
logs and customize the logs.

- enable_ec2log: Set true to enable collecting logs.

- upload_file_list: The list of files to be uploaded to S3 (default:
  `/var/log/messages*` `/var/log/aws-routed-eni/*` `/var/log/dmesg` `/tmp/kubelet.log`
  `/tmp/free.log` `/tmp/df.log` `/tmp/top.log`).

- script: The script to be executed before uploads to generate additional logs.

- s3_expire: The expiration in days for the uploaded logs (default: 30 days).

- cloudwatch_expire: The expiration in days for the CloudWatch log group for the
  Lambda function (default: 7 days).

- s3_prefix: Optional prefix for the S3 bucket name.

## Node Groups

By default, this module configures lifecycle hooks for two node groups:

- nodegroup-{cluster_name}: The main node group
- observability: The observability node group

When instances in these node groups are being terminated, the logs are
automatically saved to S3.

## IAM Permissions

The module creates and attaches the following IAM policies:

1. CloudWatch policy: Allows writing logs and completing lifecycle actions
2. S3 policy: Allows uploading logs to the S3 bucket
3. SSM policy: Allows the Lambda function to invoke SSM commands on EC2
   instances
