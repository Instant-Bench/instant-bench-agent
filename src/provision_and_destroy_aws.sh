#!/bin/bash

# Set the required variables
INSTANCE_TYPE="t2.micro"
AMI_ID="ami-0001" # Replace with your desired AMI ID
KEY_NAME="default"
# SECURITY_GROUP_ID="sg-12345678" # Replace with your security group ID
SCRIPT_PATH=$1

wait_command () {
  # Wait for the command conclusion
  until [ $(aws ssm get-command-invocation --instance-id "$INSTANCE_ID" --command-id "$1" --query 'Status' --output text) != "InProgress" ]; do 
    echo "$1 command is still running!"
    sleep 1
  done
}

# Create the EC2 instance
INSTANCE_ID=$(aws ec2 run-instances \
  --image-id $AMI_ID \
  # --security-group-ids $SECURITY_GROUP_ID \
  --user-data file://setup_node.sh \
  --instance-type $INSTANCE_TYPE \
  --key-name $KEY_NAME \
  --query 'Instances[0].InstanceId' \
  --output text)

echo "Instance $INSTANCE_ID created"

# Wait for the instance to be running
aws ec2 wait instance-running --instance-ids $INSTANCE_ID

echo "Instance $INSTANCE_ID is running."

# Get the public IP address of the instance
PUBLIC_IP=$(aws ec2 describe-instances \
  --instance-ids "$INSTANCE_ID" \
  --query 'Reservations[*].Instances[*].PublicIpAddress' \
  --output text)

scp -i ./default.pem $SCRIPT_PATH ec2-user@"$PUBLIC_IP":~/

echo "Running benchmark"
# Run the script from the API on the instance
RUN_COMMAND_ID=$(aws ssm send-command \
  --instance-ids $INSTANCE_ID \
  --document-name "AWS-RunShellScript" \
  --parameters commands=["curl -s $SCRIPT_URL | node"])

wait_command $RUN_COMMAND_ID;

# Get the output of the script
OUTPUT=$(aws ssm get-command-invocation \
  --command-id $COMMAND_ID \
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
