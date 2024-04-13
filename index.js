#!/usr/bin/env node
// Usage: ib $BINARY $ENTRYPOINT
// Usage: ib node index.js
// Usage: ib node index.js --copy-fs
const { parseArgs, debuglog } = require('node:util')
const which = require('which')
const fs = require('node:fs');
const path = require('node:path');

const log = debuglog('ib')

const options = {
  'copy-fs': {
    type: 'boolean',
    short: 'c',
  },
}

const { values, positionals } = parseArgs({ options, strict: true, allowPositionals: true })

function printUsageAndExit() {
  console.log(`
  Usage: ib $BINARY $ENTRYPOINT [--copy-fs]
  `)
  process.exit(1);
}

if (positionals.length !== 2) {
  printUsageAndExit();
}

const [binary, entrypoint] = positionals

const binaryPath = which.sync(binary)
const entrypointPath = path.resolve(entrypoint)

log('Creating benchmark folder')
const tmpFolder = fs.mkdtempSync('./.ib')

if (values['copy-fs']) {
} else {
  fs.copyFileSync(binaryPath, tmpFolder)
  fs.copyFileSync(entrypointPath, tmpFolder)
}
