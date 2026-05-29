# ── Stage 1: Development ────────────────────────────────────────
FROM node:22-slim AS development

# Install native dependencies used by monitoring collectors.
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      python3 make g++ libpcap-dev iputils-ping wget && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy workspace manifests first (for layer caching)
COPY package.json package-lock.json ./
COPY server/package.json server/
COPY client/package.json client/

RUN npm ci

# Keep simulator sources in the development image so the optional Docker
# Compose simulator service can run without bind-mounting the repository.
COPY simulator/ simulator/

ENV NODE_ENV=development
ENV PORT=3000
ENV DB_PATH=/app/data/netmonitor-dev.db
ENV CAPTURE_ENABLED=false

EXPOSE 3000
EXPOSE 5173
EXPOSE 2055/udp

CMD ["npm", "run", "dev:server"]

# ── Stage 2: Build ──────────────────────────────────────────────
FROM node:22-slim AS builder

# Install build tools for native modules (better-sqlite3, cap)
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      python3 make g++ libpcap-dev && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy workspace manifests first (for layer caching)
COPY package.json package-lock.json ./
COPY server/package.json server/
COPY client/package.json client/

# Install all dependencies (including dev for building)
RUN npm ci

# Copy source files
COPY server/ server/
COPY client/ client/

# Build server (TypeScript → JavaScript)
RUN npm run build:server

# Build client (React → static files) into server/dist/public
RUN npm run build:client

# ── Stage 3: Production ────────────────────────────────────────
FROM node:22-slim AS production

# Install runtime dependencies and build tools for native modules
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      libpcap-dev wget python3 make g++ iputils-ping && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy workspace manifests
COPY package.json package-lock.json ./
COPY server/package.json server/

# Install production dependencies only (needs build tools for native addons)
RUN npm ci --omit=dev -w server && \
    npm cache clean --force && \
    apt-get purge -y python3 make g++ && \
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/*

# Copy compiled server code (includes client build in dist/public/)
COPY --from=builder /app/server/dist server/dist/

# Create data directory for SQLite
RUN mkdir -p /app/data

# Expose ports
EXPOSE 3000
EXPOSE 2055/udp

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:3000/health || exit 1

# Environment defaults
ENV NODE_ENV=production
ENV PORT=3000
ENV DB_PATH=/app/data/netmonitor.db

# Start the server
CMD ["node", "server/dist/index.js"]
