# Instant Bench Agent Examples

This directory contains examples demonstrating how to use the Instant Bench Agent.

## Simple Benchmark

`bench.js` - A simple benchmark that measures the time to run a loop with assertions.

To run this example:

```bash
cd example
../cli/ib-agent-cli 'node bench.js'
```

or

```bash
ib-agent-cli --command='node example/bench.js'
```

## Directory with Dependencies Example

This example demonstrates how to run a benchmark with dependencies using the folder option.

Files:
- `main.js` - The main entry point that requires other modules
- `helper.js` - A helper module that provides calculation functionality
- `utils.js` - A utilities module used by the helper

To run this example by copying the entire directory:

```bash
# The --folder option is the recommended way to run benchmarks with dependencies
ib-agent-cli --folder=./example --command='node main.js'
```

This will copy the entire example directory to the benchmark environment, preserving all dependencies.
