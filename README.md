# instant-bench-agent

Set up AWS CLI and configure your AWS credentials:

```bash
$ pip install awscli
$ aws configure
```

Create an IAM role with permissions to create and destroy EC2 instances and execute the necessary AWS commands. See IAM.json

Fetch the `default.pem` from AWS KeyPair
