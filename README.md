# Instant Bench Agent

The Instant Bench Agent is a versatile tool that executes commands received via IPC/CLI/HTTP and runs the provided script on either a fresh new dedicated instance or an already running dedicated instance. It then pipes the output back to the communication socket.

## Installation

```console
$ make install
Building ib-agent-cli...
Installing ib-agent-cli to /usr/local/bin...
```

## CLI

Currently, this agent supports only CLI commands through a Go script. Therefore, having Go installed is required.

The CLI requires two positional arguments:

* `$BINARY` - The binary to execute the entry point of your benchmark. For example, if you wish to benchmark a custom version of Node.js, provide the binary path like `./node` or simply `node` if using an official version available in `$PATH`.
* `$ENTRYPOINT` - The benchmark script.

```console
$ ib-agent-cli node ./bench.js
```

This command performs the following steps:

1. Creates four resources on AWS (KeyPair, TLSPrivateKey, SecurityGroup, EC2).
2. Executes the provided `$BINARY $ENTRYPOINT`.
3. Pipes the output to the console.
4. Destroys the created resources.

**Note:** In case of failures, remember to execute `terraform destroy` inside the `aws` folder.

## AWS Setup

Before using the Instant Bench Agent with AWS, set up AWS CLI and configure your AWS credentials:

```bash
$ pip install awscli
$ aws configure
```

> Ensure you are using an IAM role with sufficient permissions to create and destroy EC2 instances.
