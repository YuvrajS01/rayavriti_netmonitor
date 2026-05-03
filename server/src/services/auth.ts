const crypto = require('crypto');
const db = require('./database');

const ACCESS_TTL_SECONDS = 900;
const REFRESH_TTL_MS = 7 * 24 * 60 * 60 * 1000;
const JWT_SECRET = process.env.JWT_SECRET || 'rayavriti-dev-secret-change-me';

const refreshTokens = new Map();
const revokedAccessTokens = new Map();

function hashPassword(password) {
  return crypto.createHash('sha256').update(password).digest('hex');
}

function safeEquals(a, b) {
  const left = Buffer.from(a);
  const right = Buffer.from(b);
  if (left.length !== right.length) {
    return false;
  }
  return crypto.timingSafeEqual(left, right);
}

function base64url(input) {
  return Buffer.from(input)
    .toString('base64')
    .replace(/=/g, '')
    .replace(/\+/g, '-')
    .replace(/\//g, '_');
}

function fromBase64url(input) {
  const normalized = input.replace(/-/g, '+').replace(/_/g, '/');
  const padded = normalized + '='.repeat((4 - (normalized.length % 4)) % 4);
  return Buffer.from(padded, 'base64').toString('utf8');
}

function signJwt(payload) {
  const header = { alg: 'HS256', typ: 'JWT' };
  const encodedHeader = base64url(JSON.stringify(header));
  const encodedPayload = base64url(JSON.stringify(payload));
  const body = `${encodedHeader}.${encodedPayload}`;
  const signature = crypto
    .createHmac('sha256', JWT_SECRET)
    .update(body)
    .digest('base64')
    .replace(/=/g, '')
    .replace(/\+/g, '-')
    .replace(/\//g, '_');

  return `${body}.${signature}`;
}

function verifyJwt(token) {
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
    .createHmac('sha256', JWT_SECRET)
    .update(body)
    .digest('base64')
    .replace(/=/g, '')
    .replace(/\+/g, '-')
    .replace(/\//g, '_');

  if (!safeEquals(expected, sig)) {
    return null;
  }

  try {
    const payload = JSON.parse(fromBase64url(encodedPayload));
    if (!payload.exp || Date.now() / 1000 >= payload.exp) {
      return null;
    }
    return payload;
  } catch (_error) {
    return null;
  }
}

function createAccessToken(user) {
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

function createRefreshToken(user) {
  const token = crypto.randomBytes(48).toString('hex');
  refreshTokens.set(token, {
    userId: String(user.id),
    username: user.username,
    role: user.role,
    expiresAt: Date.now() + REFRESH_TTL_MS
  });
  return token;
}

function buildLoginPayload(user) {
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

function login(username, password) {
  const user = db.getUserByUsername(username);
  if (!user) {
    return null;
  }

  const hashed = hashPassword(password);
  if (!safeEquals(user.password_hash, hashed)) {
    return null;
  }

  return buildLoginPayload(user);
}

function refresh(refreshToken) {
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

function getSession(token) {
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

function logout(accessToken, refreshToken) {
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

function extractToken(req) {
  const authHeader = req.headers.authorization || '';
  if (authHeader.startsWith('Bearer ')) {
    return authHeader.slice(7);
  }
  return req.headers['x-session-token'] || null;
}

module.exports = {
  login,
  refresh,
  getSession,
  logout,
  extractToken
};

export {};
