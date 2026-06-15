#!/usr/bin/env bash
set -euo pipefail

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${GREEN}[setup]${NC} $*"; }
warn() { echo -e "${YELLOW}[setup]${NC} $*"; }
info() { echo -e "${BLUE}[setup]${NC} $*"; }

# Defaults
MODE="${1:-dev}"
COMPOSE_FILE="docker-compose.yml"
ENV_FILE=".env"
ENV_EXAMPLE=".env.example"

case "$MODE" in
  dev|development)
    COMPOSE_FILE="docker-compose.dev.yml"
    ENV_FILE=".env.dev"
    ENV_EXAMPLE=".env.dev.example"
    ;;
  prod|production)
    COMPOSE_FILE="docker-compose.yml"
    ENV_FILE=".env"
    ENV_EXAMPLE=".env.example"
    ;;
  *)
    echo "Usage: ./setup.sh [dev|prod]"
    exit 1
    ;;
esac

log "Starting Rayavriti NetMonitor setup (mode: $MODE)"

# Check Docker
if ! command -v docker &> /dev/null; then
  echo "Docker is required but not installed. Please install Docker first."
  exit 1
fi

if ! docker compose version &> /dev/null; then
  echo "Docker Compose is required but not available. Please install Docker Compose."
  exit 1
fi

# Create env file if missing
if [[ ! -f "$ENV_FILE" ]]; then
  log "Creating $ENV_FILE from $ENV_EXAMPLE"
  cp "$ENV_EXAMPLE" "$ENV_FILE"

  if [[ "$MODE" == "prod" || "$MODE" == "production" ]]; then
    # Generate secure secrets for production
    JWT_SECRET=$(openssl rand -base64 32 2>/dev/null || head -c 32 /dev/urandom | base64)
    ADMIN_PASSWORD=$(openssl rand -base64 16 2>/dev/null || head -c 16 /dev/urandom | base64)

    sed -i "s/^JWT_SECRET=.*/JWT_SECRET=$JWT_SECRET/" "$ENV_FILE"
    sed -i "s/^ADMIN_PASSWORD=.*/ADMIN_PASSWORD=$ADMIN_PASSWORD/" "$ENV_FILE"

    log "Generated secure JWT_SECRET and ADMIN_PASSWORD"
    warn "Save these credentials:"
    echo "  Admin username: admin"
    echo "  Admin password: $ADMIN_PASSWORD"
    echo "  JWT_SECRET: $JWT_SECRET"
  fi
else
  info "$ENV_FILE already exists, skipping generation"
fi

# Start services
log "Starting services with $COMPOSE_FILE..."
docker compose -f "$COMPOSE_FILE" up -d --build

# Wait for health
log "Waiting for services to be healthy..."
sleep 5

# Check backend health
for i in {1..30}; do
  if curl -sf "http://localhost:3000/health" >/dev/null 2>&1; then
    log "Backend is healthy!"
    break
  fi
  if [[ $i -eq 30 ]]; then
    warn "Backend health check timed out. Check logs: docker compose -f $COMPOSE_FILE logs backend"
  fi
  sleep 2
done

# Print access info
echo ""
log "Setup complete! Access the dashboard at:"
if [[ "$MODE" == "dev" || "$MODE" == "development" ]]; then
  echo "  Frontend: http://localhost:5173"
  echo "  Backend API: http://localhost:3000"
  echo "  Default login: admin / admin123"
else
  echo "  Dashboard: http://localhost:3000"
fi
echo ""
log "Useful commands:"
echo "  View logs:     docker compose -f $COMPOSE_FILE logs -f"
echo "  Stop:          docker compose -f $COMPOSE_FILE down"
echo "  Restart:       docker compose -f $COMPOSE_FILE restart"