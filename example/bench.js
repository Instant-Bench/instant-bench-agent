const assert = require('node:assert')

console.time('Example')
for (let i = 1; i < 10e4; ++i) {
  assert.ok(i + 1);
}
console.timeEnd('Example')
