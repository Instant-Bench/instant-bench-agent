# instant-bench-agent

An agent that receives commands via IPC/CLI/HTTP and runs the given script on either:

* A fresh new dedicated instance or
* A up-and-running dedicated instance

And pipe the output to the communication socket.

## Install

```console
$ sudo make install
Building ib-agent-cli...
Installing ib-agent-cli to /usr/local/bin...
```

## CLI

Currently, this agent supports only CLI commands through a `golang` script.
Therefore, having `golang` is required.

The CLI receives two positional arguments:

* $BINARY - A binary to run the entrypoint of your benchmark. For instance, if you want to benchmark a custom version of Node.js, pass the binary: `./node` or simply `node` if you want to use an official version (available in $PATH).
* $ENTRYPOINT - The benchmark script.

```console
$ ib-agent-cli node ./bench.js
```

**Important**: In case of failures, remember to `terraform destroy` inside `aws` folder.

## AWS

Set up AWS CLI and configure your AWS credentials:

```bash
$ pip install awscli
$ aws configure
```

> Remember to use an IAM role with sufficient permissions to create and destroy EC2 instances.
