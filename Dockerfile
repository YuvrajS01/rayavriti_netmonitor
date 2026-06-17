# ── Stage 1: Build React client ─────────────────────────────────
FROM node:24-alpine AS client-builder

WORKDIR /app

COPY package.json package-lock.json ./
COPY client/package.json ./client/
RUN npm ci --workspace client

COPY client/ ./client/
RUN npm run build -w client

# ── Stage 2: Build Go backend (production builder) ──────────────
FROM golang:1.26-alpine AS go-builder

RUN apk add --no-cache gcc musl-dev libpcap-dev

WORKDIR /app

COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download

COPY backend/ ./backend/

RUN cd backend && CGO_ENABLED=1 go build -tags pcap -ldflags="-s -w" -o /netmonitor ./cmd/server

# ── Stage 3: Production image (default target) ──────────────────
FROM alpine:3.21 AS production

RUN apk add --no-cache ca-certificates libpcap tzdata wget tcpdump

COPY --from=go-builder /netmonitor /usr/local/bin/netmonitor
COPY --from=client-builder /app/client/dist /app/public

WORKDIR /app

RUN mkdir -p /app/data/logs

EXPOSE 3000
EXPOSE 2055/udp

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:3000/health || exit 1

ENV APP_ENV=production
ENV PORT=3000

ENTRYPOINT ["netmonitor"]

# ── Stage 4: Go backend development (hot reload) ───────────────
# Only built when targeting: docker build --target development .
FROM golang:1.26-alpine AS development

RUN apk add --no-cache gcc musl-dev libpcap-dev make git bash tcpdump

WORKDIR /app

RUN go install github.com/air-verse/air@latest

COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download

COPY backend/ ./backend/

ENV APP_ENV=development
ENV PORT=3000
ENV DATABASE_URL=postgres://netmonitor:netmonitor@localhost:5433/netmonitor?sslmode=disable
ENV CAPTURE_ENABLED=false

EXPOSE 3000
EXPOSE 5173
EXPOSE 2055/udp

CMD ["air", "-c", ".air.toml"]

# ── Stage 5: Client development (Vite dev server) ──────────────
# Only built when targeting: docker build --target client-development .
FROM node:24-alpine AS client-development

WORKDIR /app

COPY package.json package-lock.json ./
COPY client/package.json ./client/
RUN npm ci --workspace client
RUN npm install --force @rolldown/binding-linux-x64-musl

COPY client/ ./client/

EXPOSE 5173

CMD ["npm", "--workspace", "client", "run", "dev", "--", "--host", "0.0.0.0"]
