# Save EKS node logs on S3 when an node is terminated

This module is an option in the cluster module if the variable *enable_ec2log*
is set. It push the logs in the EKS node EC2 instance to an S3 before the
instance is terminated for troubleshooting purposes.

## Backgroupd

Sometimes when an EKS cluster experiences critical issues, it terminates the
nodes in trouble and restarts new ones. The logs in the terminated nodes will be
removed as the result and will not been available for troubleshooting. We need a
way to keep the logs.

## How it works

The following main AWS resources are created for each cluster and involved in
the procedure:

- An Auto Scaling Group (ASG) terminating hook to trigger the Lambda function.

- A Lambda function which responds the terminating event. It passes the
  parameters and executes the SSM document.

- An SSM document which uploads the logs to the S3 bucket.

- An S3 bucket to save the logs.

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

- ec2log_file_list: The list of files to be uploaded to S3.

- ec2log_script: The script to be executed before uploads.

- ec2log_s3_expire: The expiration in days for the uploaded logs.

- ec2log_cw_expire: The expiration in days for the CloudWatch log group for the
  Lambda function.
