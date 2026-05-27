import crypto from 'crypto';

const SCRYPT_KEYLEN = 64;

/**
 * Hash a password using scrypt with a random 32-byte salt.
 * Returns `salt:hash` (both hex-encoded).
 */
export function hashPassword(password: string): string {
  const salt = crypto.randomBytes(32).toString('hex');
  const hash = crypto.scryptSync(password, salt, SCRYPT_KEYLEN).toString('hex');
  return `${salt}:${hash}`;
}

/**
 * Hash a password using legacy SHA-256 (for comparison only — do NOT use for new hashes).
 */
export function hashPasswordLegacy(password: string): string {
  return crypto.createHash('sha256').update(password).digest('hex');
}

/**
 * Verify a password against a stored hash.
 * Supports both formats:
 *   - scrypt: `salt:hash`  (contains a colon)
 *   - legacy SHA-256: plain hex string (no colon)
 *
 * Uses timing-safe comparison in both paths.
 */
export function verifyPassword(password: string, stored: string): boolean {
  if (stored.includes(':')) {
    // scrypt format: salt:hash
    const [salt, storedHash] = stored.split(':');
    const derived = crypto.scryptSync(password, salt, SCRYPT_KEYLEN).toString('hex');
    return safeEquals(derived, storedHash);
  }

  // Legacy SHA-256 fallback
  const sha256 = crypto.createHash('sha256').update(password).digest('hex');
  return safeEquals(sha256, stored);
}

/**
 * Timing-safe string comparison.
 */
function safeEquals(a: string, b: string): boolean {
  const left = Buffer.from(a);
  const right = Buffer.from(b);
  if (left.length !== right.length) {
    return false;
  }
  return crypto.timingSafeEqual(left, right);
}
