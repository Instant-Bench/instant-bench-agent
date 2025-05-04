// Example helper module with a nested dependency
const utils = require('./utils');

/**
 * Calculate the sum of numbers from 1 to n
 * @param {number} n - The upper limit
 * @returns {number} - The sum
 */
function calculate(n) {
  // Use the utility function to validate input
  utils.validatePositiveInteger(n);
  
  let sum = 0;
  for (let i = 1; i <= n; i++) {
    sum += i;
  }
  return sum;
}

module.exports = {
  calculate
};
