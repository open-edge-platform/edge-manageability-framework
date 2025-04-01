# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

import json
import boto3

SSM_DOC = "orch-ec2log-${cluster_name}-term"

def lambda_handler(event, context):
    # Initialize the SSM client
    ssm_client = boto3.client('ssm')
    
    # Print the event to see its structure (useful for debugging)
    print("Received event: " + json.dumps(event, indent=2))
    
    # Extract the ASG name and EC2 instance ID from the event
    asg_name = None
    ec2_instance_id = None
    lifecycle_hook_name = None
    
    # Check if the event is from SNS
    if 'Records' in event and 'Sns' in event['Records'][0]:
        # Parse the SNS message
        message = json.loads(event['Records'][0]['Sns']['Message'])
        
        # Extract the ASG name and EC2 instance ID from the SNS message
        asg_name = message.get('AutoScalingGroupName')
        ec2_instance_id = message.get('EC2InstanceId')
        lifecycle_hook_name = message.get('LifecycleHookName')
        
    # Check if the event is from CloudWatch Events
    elif 'detail' in event:
        # Extract the ASG name and EC2 instance ID from the CloudWatch event
        asg_name = event['detail'].get('AutoScalingGroupName')
        ec2_instance_id = event['detail'].get('EC2InstanceId')
        lifecycle_hook_name = event['detail'].get('LifecycleHookName')
    else:
        print("Event structure not recognized.")
        return {
            'statusCode': 400,
            'body': json.dumps('Event structure not recognized.')
        }
    
    # Log the ASG name and EC2 instance ID
    print(f"Auto Scaling Group Name: {asg_name}")
    print(f"EC2 Instance ID: {ec2_instance_id}")
    print(f"Lifecycle Hook Name: {lifecycle_hook_name}")
    
    # If the EC2 instance ID and ASG name are available, call the SSM document
    if ec2_instance_id and asg_name:
        try:
            response = ssm_client.send_command(
                InstanceIds=[ec2_instance_id],
                DocumentName=SSM_DOC,
                Parameters={
                    'ASGNAME': [asg_name],
                    'LIFECYCLEHOOKNAME': [lifecycle_hook_name],
                    'BACKUPFILES': ['${upload_file_list}'],
                    'SCRIPT': ['${script}']
                }
            )
            print(f"SSM Document '" + SSM_DOC + "' called successfully: {response}")
        except Exception as e:
            print(f"Error calling SSM Document '" + SSM_DOC + "'. ")
            raise e
    else:
        print("EC2 Instance ID or ASG Name not found in the event.")
    
    return {
        'statusCode': 200,
        'body': json.dumps('Lambda function executed successfully!')
    }
