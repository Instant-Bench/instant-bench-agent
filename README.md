# Instant Bench Agent

The Instant Bench Agent is a versatile tool that executes commands received via IPC/CLI/HTTP and runs the provided script on either a fresh new dedicated instance or an already running dedicated instance. It then pipes the output back to the communication socket.

> [!WARNING]
> This tool is still in development phrase. No stable version was released yet.

## Installation

```console
$ make install
Building ib-agent-cli...
Installing ib-agent-cli to /usr/local/bin...
```

## CLI

The CLI supports running benchmarks either on a newly provisioned AWS instance or on an existing machine via SSH.

### Basic Usage

You can use the CLI in two ways:

1. With a command directly as a positional argument:
   ```console
   $ ib-agent-cli 'node ./bench.js'
   ```

2. With the `--command` flag:
   ```console
   $ ib-agent-cli --command='node bench.js'
   ```

The tool will automatically infer the binary from the command (in this case, `node`). If it can't find the binary in your PATH, it will rely on the remote system having it installed.

### Running on a New AWS Instance

By default, the command performs the following steps:

1. Creates four resources on AWS (KeyPair, TLSPrivateKey, SecurityGroup, EC2).
2. Executes the provided command.
3. Pipes the output to the console.
4. Destroys the created resources.

**Note:** In case of failures, remember to execute `terraform destroy` inside the `aws` folder.

### Running on an Existing Machine

You can run the benchmark on an existing machine by providing the `--host` parameter:

```console
$ ib-agent-cli --host=192.168.1.100 --ssh-key=~/.ssh/id_rsa --command='node bench.js'
```

This will:
1. Copy the specified files to the remote machine
2. Execute the command on the remote machine
3. Pipe the output back to your console

### Copying a Directory with Dependencies

To copy an entire directory with all your dependencies, use the `--folder` flag:

```console
$ ib-agent-cli --folder=./my-project --command='node index.js'
```

This will recursively copy the entire directory to the benchmark environment, preserving the directory structure.

### Available Options

```
Usage: ib-agent-cli [options] [COMMAND] | [--command="custom command"]

Options:
  --host=IP               Run on existing machine with this IP address
  --ssh-key=PATH          Path to SSH private key for connecting to existing machine
  --ssh-user=USERNAME     SSH username for connecting to existing machine (default: ubuntu)
  --folder=PATH           Path to folder containing all dependencies to be copied
  --command=COMMAND       Custom command to run on the instance
  --instance-type=TYPE    AWS instance type to use (default: t2.micro)
```

## AWS Setup

Before using the Instant Bench Agent with AWS, set up AWS CLI and configure your AWS credentials:

```bash
$ pip install awscli
$ aws configure
```

> Ensure you are using an IAM role with sufficient permissions to create and destroy EC2 instances.
