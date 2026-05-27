import crypto from 'crypto';

import db from './database';

// Re-export from the shared password utility so consumers can
// `import { hashPassword, verifyPassword } from './auth'` if needed.
export { hashPassword, verifyPassword } from './password';

// Use a local import alias for internal use
import { verifyPassword as _verifyPassword } from './password';

// ── Configuration ───────────────────────────────────────────────
const ACCESS_TTL_SECONDS = 900;
const REFRESH_TTL_MS = 7 * 24 * 60 * 60 * 1000;

const JWT_SECRET = process.env.JWT_SECRET;
if (!JWT_SECRET && process.env.NODE_ENV === 'production') {
  throw new Error('JWT_SECRET environment variable is required in production');
}
const SECRET: string = JWT_SECRET || 'rayavriti-dev-secret-change-me';

// ── In-memory token stores ──────────────────────────────────────

interface RefreshSession {
  userId: string;
  username: string;
  role: string;
  expiresAt: number;
}

const refreshTokens = new Map<string, RefreshSession>();
const revokedAccessTokens = new Map<string, number>();

// ── JWT helpers (hand-rolled HS256) ─────────────────────────────

function base64url(input: string): string {
  return Buffer.from(input)
    .toString('base64')
    .replace(/=/g, '')
    .replace(/\+/g, '-')
    .replace(/\//g, '_');
}

function fromBase64url(input: string): string {
  const normalized = input.replace(/-/g, '+').replace(/_/g, '/');
  const padded = normalized + '='.repeat((4 - (normalized.length % 4)) % 4);
  return Buffer.from(padded, 'base64').toString('utf8');
}

interface JwtPayload {
  sub?: string;
  username?: string;
  role?: string;
  type?: string;
  iat?: number;
  exp?: number;
  jti?: string;
  [key: string]: unknown;
}

function signJwt(payload: JwtPayload): string {
  const header = { alg: 'HS256', typ: 'JWT' };
  const encodedHeader = base64url(JSON.stringify(header));
  const encodedPayload = base64url(JSON.stringify(payload));
  const body = `${encodedHeader}.${encodedPayload}`;
  const signature = crypto
    .createHmac('sha256', SECRET)
    .update(body)
    .digest('base64')
    .replace(/=/g, '')
    .replace(/\+/g, '-')
    .replace(/\//g, '_');

  return `${body}.${signature}`;
}

function safeEquals(a: string, b: string): boolean {
  const left = Buffer.from(a);
  const right = Buffer.from(b);
  if (left.length !== right.length) {
    return false;
  }
  return crypto.timingSafeEqual(left, right);
}

function verifyJwt(token: string | null | undefined): JwtPayload | null {
  if (!token) {
    return null;
  }

  const parts = token.split('.');
  if (parts.length !== 3) {
    return null;
  }

  const [encodedHeader, encodedPayload, sig] = parts;
  const body = `${encodedHeader}.${encodedPayload}`;
  const expected = crypto
    .createHmac('sha256', SECRET)
    .update(body)
    .digest('base64')
    .replace(/=/g, '')
    .replace(/\+/g, '-')
    .replace(/\//g, '_');

  if (!safeEquals(expected, sig)) {
    return null;
  }

  try {
    const payload: JwtPayload = JSON.parse(fromBase64url(encodedPayload));
    if (!payload.exp || Date.now() / 1000 >= payload.exp) {
      return null;
    }
    return payload;
  } catch (_error) {
    return null;
  }
}

// ── Token creation ──────────────────────────────────────────────

interface UserLike {
  id: number | string;
  username: string;
  role: string;
}

function createAccessToken(user: UserLike): string {
  const now = Math.floor(Date.now() / 1000);
  return signJwt({
    sub: String(user.id),
    username: user.username,
    role: user.role,
    type: 'access',
    iat: now,
    exp: now + ACCESS_TTL_SECONDS,
    jti: crypto.randomBytes(12).toString('hex')
  });
}

function createRefreshToken(user: UserLike): string {
  const token = crypto.randomBytes(48).toString('hex');
  refreshTokens.set(token, {
    userId: String(user.id),
    username: user.username,
    role: user.role,
    expiresAt: Date.now() + REFRESH_TTL_MS
  });
  return token;
}

function buildLoginPayload(user: UserLike) {
  const accessToken = createAccessToken(user);
  const refreshToken = createRefreshToken(user);

  return {
    token: accessToken,
    accessToken,
    refreshToken,
    expiresIn: ACCESS_TTL_SECONDS,
    user: {
      id: String(user.id),
      username: user.username,
      role: user.role === 'admin' ? 'administrator' : user.role
    }
  };
}

// ── Public API ──────────────────────────────────────────────────

export function login(username: string, password: string) {
  const user: any = db.getUserByUsername(username);
  if (!user) {
    return null;
  }

  if (!_verifyPassword(password, user.password_hash)) {
    return null;
  }

  return buildLoginPayload(user);
}

export function refresh(refreshToken: string) {
  const session = refreshTokens.get(refreshToken);
  if (!session) {
    return null;
  }

  if (Date.now() > session.expiresAt) {
    refreshTokens.delete(refreshToken);
    return null;
  }

  const accessToken = createAccessToken({
    id: session.userId,
    username: session.username,
    role: session.role
  });

  return {
    accessToken,
    expiresIn: ACCESS_TTL_SECONDS,
    user: {
      id: String(session.userId),
      username: session.username,
      role: session.role === 'admin' ? 'administrator' : session.role
    }
  };
}

export function getSession(token: string | null | undefined) {
  if (!token) {
    return null;
  }

  if (revokedAccessTokens.has(token)) {
    return null;
  }

  const payload = verifyJwt(token);
  if (!payload || payload.type !== 'access') {
    return null;
  }

  return {
    userId: payload.sub,
    username: payload.username,
    role: payload.role === 'administrator' ? 'admin' : payload.role
  };
}

export function logout(accessToken: string | null, refreshToken: string | null) {
  if (accessToken) {
    const payload = verifyJwt(accessToken);
    if (payload?.exp) {
      revokedAccessTokens.set(accessToken, payload.exp * 1000);
    }
  }

  if (refreshToken) {
    refreshTokens.delete(refreshToken);
  }

  const now = Date.now();
  for (const [token, expiry] of revokedAccessTokens.entries()) {
    if (expiry <= now) {
      revokedAccessTokens.delete(token);
    }
  }
}

export function extractToken(req: { headers: Record<string, string | undefined> }): string | null {
  const authHeader = req.headers.authorization || '';
  if (authHeader.startsWith('Bearer ')) {
    return authHeader.slice(7);
  }
  return req.headers['x-session-token'] || null;
}


