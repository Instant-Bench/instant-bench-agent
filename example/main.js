// Example main file that requires dependencies
const helper = require('./helper');
const assert = require('node:assert');

console.log('Starting benchmark with dependencies...');
console.time('Benchmark');

// Use the helper function
const result = helper.calculate(10);
assert.strictEqual(result, 55, 'Helper calculation should return 55');

console.timeEnd('Benchmark');
console.log('Benchmark completed successfully!');
