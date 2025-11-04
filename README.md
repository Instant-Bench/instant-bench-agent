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

The CLI supports running benchmarks either on a newly provisioned cloud instance (AWS or Hetzner Cloud) or on an existing machine via SSH.

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

**Note:** In case of failures, remember to execute `terraform destroy` inside the `aws/` folder.

### Running on a New Hetzner Cloud Instance

1. Set your Hetzner Cloud API token in the environment (the Terraform provider reads `HCLOUD_TOKEN`):

   ```bash
   export HCLOUD_TOKEN="<your_hcloud_api_token>"
   ```

2. Run your command on Hetzner Cloud, specifying server type and location:

   ```console
   $ ib-agent-cli \
     --cloud=hetzner \
     --server-type=cax11 \
     --location=fsn1 \
     --command='node bench.js'
   ```

3. Copy a project folder and use compound commands:

   ```console
   $ ib-agent-cli --cloud=hetzner --folder=./my-project \
     --command='pwd && ls && node index.js'
   ```

By default, the remote environment installs NVM and Node v22. If you need a different Node version, include it in your command, for example:

```console
$ ib-agent-cli --cloud=hetzner \
  --command='. ~/.nvm/nvm.sh && nvm install 24 && nvm use 24 && node bench.js'
```

### Running on an Existing Machine

You can run the benchmark on an existing machine by providing the `--host` parameter:

```console
$ ib-agent-cli --host=192.168.1.100 --ssh-key=~/.ssh/id_rsa --command='node bench.js'
```

This will:
1. Copy the specified files to the remote machine
2. Execute the command on the remote machine
3. Pipe the output back to your console

### Copying Directories with Dependencies

To copy an entire directory with all your dependencies, use the `--folder` flag:

```console
$ ib-agent-cli --folder=./my-project --command='node index.js'
```

This will recursively copy the entire directory to the benchmark environment, preserving the directory structure.

**Tip**: Use the `--ignore` flag to exclude directories you don't need (like `node_modules`):

```console
# Ignore node_modules and .git
$ ib-agent-cli \
  --folder=./my-project \
  --ignore=node_modules \
  --ignore=.git \
  --command='npm install && node index.js'
```

Without `--ignore`, all files and directories are copied. This gives you full control over what gets transferred.

#### Copying Multiple Folders

You can specify the `--folder` flag multiple times to copy multiple directories. This is useful when you want to benchmark multiple versions of the same project:

```console
$ ib-agent-cli \
  --cloud=hetzner \
  --folder=~/repos/os/fastify \
  --folder=~/repos/os/fastify2 \
  --command='cd fastify && node benchmark.js && cd ../fastify2 && node benchmark.js'
```

Each folder will be copied to the remote machine with its original name preserved, so you can reference them in your command. In the example above, both `fastify` and `fastify2` will be available in the benchmark environment.

**Important**: Each of the 3 benchmark runs starts fresh from the benchmark directory, so you can use relative paths (like `cd fastify && npm run benchmark`) without worrying about working directory state between runs.

**Real-world example**: Comparing performance of two different versions of a project:

```console
$ ib-agent-cli \
  --cloud=hetzner \
  --server-type=cx22 \
  --folder=~/repos/os/fastify \
  --folder=~/repos/os/fastify2 \
  --command='echo "Testing fastify v1:" && cd fastify && npm install && node benchmark.js && echo "Testing fastify v2:" && cd ../fastify2 && npm install && node benchmark.js'
```

After provisioning the machine, you'll be able to SSH into it and both folders will be available in `/root/benchmark/` (or `/home/ubuntu/benchmark/` if using an existing machine with `--host`).

### High-Performance Testing

For performance-critical benchmarks, use the `--high-tier` flag to automatically provision powerful dedicated servers:

```console
$ ib-agent-cli \
  --cloud=hetzner \
  --high-tier \
  --folder=~/repos/os/fastify \
  --command='node benchmark.js'
```

The `--high-tier` flag automatically selects:
- **Hetzner**: `ccx33` (8 dedicated vCPUs, 32GB RAM)
- **AWS**: `c5.2xlarge` (8 vCPUs, 16GB RAM)

You can still override with `--server-type` or `--instance-type` if needed.

**Combining multiple folders with high-tier performance**:

```console
$ ib-agent-cli \
  --cloud=hetzner \
  --high-tier \
  --folder=~/repos/os/fastify \
  --folder=~/repos/os/fastify2 \
  --command='cd fastify && npm install && node benchmark.js && cd ../fastify2 && npm install && node benchmark.js'
```

This gives you the best of both worlds: comparing multiple versions on dedicated high-performance hardware.

### Available Options

```
Usage: ib-agent-cli [options] [COMMAND] | (--command="custom command")

Options:
  --host=IP               Run on existing machine with this IP address
  --ssh-key=PATH          Path to SSH private key for connecting to existing machine
  --ssh-user=USERNAME     SSH username for connecting to existing machine (default: ubuntu)
  --folder=PATH           Path to folder containing all dependencies to be copied (can be specified multiple times)
  --ignore=DIR            Directory name to ignore when copying folders (e.g., node_modules, .git). Can be specified multiple times.
  --command=COMMAND       Custom command to run on the instance
  --instance-type=TYPE    AWS instance type to use (default: t2.micro)
  --cloud=PROVIDER        Cloud provider to use: aws or hetzner (default: aws)
  --server-type=TYPE      Hetzner server type (for --cloud=hetzner, default: cax11)
  --location=LOC          Hetzner location (for --cloud=hetzner, default: fsn1)
  --high-tier             Use high-performance server types (Hetzner: ccx33, AWS: c5.2xlarge)
  --debug                 Enable debug logging
```

## Cloud Setup

### AWS

Before using the Instant Bench Agent with AWS, set up AWS CLI and configure your AWS credentials:

```bash
$ pip install awscli
$ aws configure
```

> Ensure you are using an IAM role with sufficient permissions to create and destroy EC2 instances.

### Hetzner Cloud

Export your Hetzner Cloud API token before using `--cloud=hetzner`:

```bash
export HCLOUD_TOKEN="<your_hcloud_api_token>"
```

## Behavior and Notes

- **Auto file detection**: The CLI scans your command for referenced files and copies them to the remote environment. Use `--folder` to copy an entire project.
- **Multiple folders**: Specify `--folder` multiple times to copy multiple directories to the remote machine.
- **Selective folder copying**: Use `--ignore` to skip specific directories when copying folders (e.g., `--ignore=node_modules --ignore=.git`).
- **High-tier performance**: Use `--high-tier` to automatically provision powerful dedicated servers (Hetzner: ccx33, AWS: c5.2xlarge) for accurate performance benchmarks.
- **Compound commands**: Commands joined with `&&` are supported; binaries from each part are handled.
- **Destruction**: Resources are destroyed automatically after running. For manual cleanup or on errors:
  - AWS: `cd aws && terraform destroy`
  - Hetzner: `cd hetzner && terraform destroy`
- **Debugging**: Use `--debug` to see detailed logs and remote output around `BENCHMARK_START/BENCHMARK_END`.
