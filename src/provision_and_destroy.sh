#!/bin/bash

# Set the required variables
INSTANCE_TYPE="t2.micro"
AMI_ID="ami-12345678" # Replace with your desired AMI ID
KEY_NAME="my-key" # Replace with your key pair name
SECURITY_GROUP_ID="sg-12345678" # Replace with your security group ID
SCRIPT_URL="https://example.com/api/script" # Replace with your API URL

# Create the EC2 instance
INSTANCE_ID=$(aws ec2 run-instances \
  --image-id $AMI_ID \
  --instance-type $INSTANCE_TYPE \
  --key-name $KEY_NAME \
  --security-group-ids $SECURITY_GROUP_ID \
  --query 'Instances[0].InstanceId' \
  --output text)

echo "Instance $INSTANCE_ID created"

# Wait for the instance to be running
aws ec2 wait instance-running --instance-ids $INSTANCE_ID

echo "Instance $INSTANCE_ID is running"

# Run the script from the API on the instance
aws ssm send-command \
  --instance-ids $INSTANCE_ID \
  --document-name "AWS-RunShellScript" \
  --parameters commands=["curl -s $SCRIPT_URL | node"]

# Wait for the script to finish executing
sleep 10 # Adjust the sleep time as needed

# Get the output of the script
OUTPUT=$(aws ssm get-command-invocation \
  --command-id $(aws ssm list-command-invocations \
    --instance-id $INSTANCE_ID \
    --query "CommandInvocations[0].CommandId" \
    --output text) \
  --instance-id $INSTANCE_ID \
  --query "StandardOutputContent" \
  --output text)

echo "Script output:"
echo "$OUTPUT"

# Terminate the EC2 instance
aws ec2 terminate-instances --instance-ids $INSTANCE_ID

echo "Instance $INSTANCE_ID terminated"
