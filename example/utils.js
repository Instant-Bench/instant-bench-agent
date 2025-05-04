// Example utilities module
/**
 * Validate that a value is a positive integer
 * @param {any} value - The value to validate
 * @throws {Error} - If the value is not a positive integer
 */
function validatePositiveInteger(value) {
  if (typeof value !== 'number' || !Number.isInteger(value) || value <= 0) {
    throw new Error(`Expected a positive integer, got ${value}`);
  }
}

module.exports = {
  validatePositiveInteger
};
