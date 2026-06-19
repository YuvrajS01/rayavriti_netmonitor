#!/usr/bin/env bash
set -euo pipefail

# ─────────────────────────────────────────────────────────────────
#  Rayavriti NetMonitor — One-shot bootstrap
#
#  Usage (run on any fresh system):
#
#    curl -fsSL https://raw.githubusercontent.com/YuvrajS01/rayavriti_netmonitor/main/bootstrap.sh | bash -s -- --dev --docker
#
#  CLI flags (skip interactive prompts):
#    --dev / --prod        Deployment mode
#    --docker / --bare-metal  Runtime
#    --dir <path>          Install directory
#
#  Or clone first and run manually:
#
#    git clone https://github.com/YuvrajS01/rayavriti_netmonitor.git
#    cd rayavriti_netmonitor
#    ./bootstrap.sh
# ─────────────────────────────────────────────────────────────────

REPO_URL="https://github.com/YuvrajS01/rayavriti_netmonitor.git"
REPO_NAME="rayavriti_netmonitor"
DEFAULT_INSTALL_DIR="$HOME/projects"

# ── CLI flags ────────────────────────────────────────────────────
DEPLOY_MODE="${DEPLOY_MODE:-}"
RUNTIME="${RUNTIME:-}"
INSTALL_DIR="${INSTALL_DIR:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dev)          DEPLOY_MODE="dev"; shift ;;
    --prod)         DEPLOY_MODE="prod"; shift ;;
    --docker)       RUNTIME="docker"; shift ;;
    --bare-metal)   RUNTIME="bare-metal"; shift ;;
    --dir)          INSTALL_DIR="$2"; shift 2 ;;
    *)              shift ;;
  esac
done

# ── Colors & helpers ─────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log()   { echo -e "${GREEN}[✓]${NC} $*"; }
info()  { echo -e "${BLUE}[i]${NC} $*"; }
warn()  { echo -e "${YELLOW}[!]${NC} $*"; }
err()   { echo -e "${RED}[✗]${NC} $*"; }
header() {
  echo ""
  echo -e "${CYAN}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "${CYAN}${BOLD}  $*${NC}"
  echo -e "${CYAN}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo ""
}

prompt() {
  local var_name="$1"
  local label="$2"
  local default="${3:-}"
  local prompt_str="  ${label}"
  [[ -n "$default" ]] && prompt_str+=" [${default}]"
  prompt_str+=": "
  local value=""
  if [[ -t 0 ]]; then
    read -rp "$prompt_str" value
  elif [[ -r /dev/tty ]]; then
    read -rp "$prompt_str" value < /dev/tty || true
  else
    echo "$default"
    return
  fi
  echo "${value:-$default}"
}

secret() {
  local var_name="$1"
  local label="$2"
  local prompt_str="  ${label}: "
  local value=""
  if [[ -t 0 ]]; then
    read -rsp "$prompt_str" value
  elif [[ -r /dev/tty ]]; then
    read -rsp "$prompt_str" value < /dev/tty || true
  else
    echo ""
    return
  fi
  echo ""
  echo "$value"
}

# ── Banner ───────────────────────────────────────────────────────
header "Rayavriti NetMonitor — Bootstrap Installer"

echo -e "  This script will:"
echo -e "  ${GREEN}1.${NC} Check system prerequisites"
echo -e "  ${GREEN}2.${NC} Clone the repository"
echo -e "  ${GREEN}3.${NC} Configure environment variables"
echo -e "  ${GREEN}4.${NC} Install dependencies & start services"
echo ""

# ── Step 0: Interactive choices ─────────────────────────────────
header "Step 1/5 — Configuration"

if [[ -z "$DEPLOY_MODE" ]]; then
  DEPLOY_MODE=$(prompt "deploy" "Deployment mode (dev/prod)" "dev")
fi
if [[ -z "$RUNTIME" ]]; then
  RUNTIME=$(prompt "runtime" "Runtime (docker/bare-metal)" "docker")
fi

if [[ "$DEPLOY_MODE" != "dev" && "$DEPLOY_MODE" != "prod" ]]; then
  err "Invalid deploy mode: $DEPLOY_MODE (must be 'dev' or 'prod')"
  exit 1
fi

if [[ "$RUNTIME" != "docker" && "$RUNTIME" != "bare-metal" ]]; then
  err "Invalid runtime: $RUNTIME (must be 'docker' or 'bare-metal')"
  exit 1
fi

if [[ -z "$INSTALL_DIR" ]]; then
  INSTALL_DIR=$(prompt "dir" "Install directory" "$DEFAULT_INSTALL_DIR/$REPO_NAME")
fi

info "Mode: ${BOLD}$DEPLOY_MODE${NC}  |  Runtime: ${BOLD}$RUNTIME${NC}  |  Dir: ${BOLD}$INSTALL_DIR${NC}"
echo ""

# ── Step 1: Prerequisites ──────────────────────────────────────
header "Step 2/5 — Checking prerequisites"

missing=()

check_cmd() {
  if command -v "$1" &>/dev/null; then
    log "$1 found: $(command -v "$1")"
    return 0
  else
    err "$1 not found"
    missing+=("$1")
    return 1
  fi
}

if [[ "$RUNTIME" == "docker" ]]; then
  check_cmd docker || true
  if docker compose version &>/dev/null 2>&1; then
    log "docker compose found: $(docker compose version --short 2>/dev/null || echo 'available')"
  else
    err "docker compose plugin not found"
    missing+=("docker compose")
    # Try docker-compose as fallback
    if command -v docker-compose &>/dev/null; then
      warn "docker-compose (standalone) found instead"
    fi
  fi
else
  # Bare-metal: check build tools
  info "Bare-metal mode — checking build dependencies..."
  check_cmd git || true
  check_cmd node || true
  check_cmd npm || true
  check_cmd go || true

  # Check for libpcap (needed for packet capture)
  if [[ "$(uname)" == "Linux" ]]; then
    if dpkg -l libpcap-dev &>/dev/null 2>&1 || rpm -q libpcap-devel &>/dev/null 2>&1; then
      log "libpcap-dev found"
    else
      warn "libpcap-dev not found (optional, needed for packet capture)"
    fi
  fi
fi

if [[ ${#missing[@]} -gt 0 ]]; then
  echo ""
  err "Missing required tools: ${missing[*]}"
  echo ""
  echo -e "  Install them and re-run this script."

  if [[ "$(uname)" == "Linux" ]]; then
    echo -e "  ${CYAN}Ubuntu/Debian:${NC}"
    echo "    sudo apt update && sudo apt install -y docker.io docker-compose-v2 git curl"
    echo ""
    echo -e "  ${CYAN}RHEL/Fedora:${NC}"
    echo "    sudo dnf install -y docker docker-compose git curl"
  elif [[ "$(uname)" == "Darwin" ]]; then
    echo -e "  ${CYAN}macOS:${NC}"
    echo "    brew install docker git"
  fi
  echo ""
  exit 1
fi

log "All prerequisites satisfied"
echo ""

# ── Step 2: Clone repo ─────────────────────────────────────────
header "Step 3/5 — Cloning repository"

if [[ -d "$INSTALL_DIR" ]]; then
  warn "Directory already exists: $INSTALL_DIR"
  PROMPT_REUSE=$(prompt "reuse" "Use existing directory? (y/n)" "y")
  if [[ "$PROMPT_REUSE" == "y" || "$PROMPT_REUSE" == "Y" ]]; then
    info "Using existing directory"
    cd "$INSTALL_DIR"
    git pull --ff-only || warn "git pull failed — using existing code"
  else
    INSTALL_DIR="${INSTALL_DIR}_$(date +%s)"
    info "Cloning to: $INSTALL_DIR"
    git clone "$REPO_URL" "$INSTALL_DIR"
    cd "$INSTALL_DIR"
  fi
else
  info "Cloning to: $INSTALL_DIR"
  git clone "$REPO_URL" "$INSTALL_DIR"
  cd "$INSTALL_DIR"
fi

log "Repository ready at: $(pwd)"
echo ""

# ── Step 3: Environment configuration ──────────────────────────
header "Step 4/5 — Configuring environment"

if [[ "$DEPLOY_MODE" == "prod" ]]; then
  ENV_FILE=".env"
  ENV_EXAMPLE=".env.example"
else
  ENV_FILE=".env.dev"
  ENV_EXAMPLE=".env.dev.example"
fi

if [[ ! -f "$ENV_FILE" ]]; then
  cp "$ENV_EXAMPLE" "$ENV_FILE"
  log "Created $ENV_FILE from template"
else
  info "$ENV_FILE already exists — skipping copy"
fi

# Configure variables interactively
echo ""
echo -e "  ${BOLD}Configure environment variables:${NC}"
echo -e "  (Press Enter to keep default values shown in brackets)"
echo ""

if [[ "$DEPLOY_MODE" == "prod" ]]; then
  # Production: require JWT_SECRET and ADMIN_PASSWORD
  CURRENT_JWT=$(grep -oP '^JWT_SECRET=\K.*' "$ENV_FILE" 2>/dev/null || echo "")
  CURRENT_ADMIN_PW=$(grep -oP '^ADMIN_PASSWORD=\K.*' "$ENV_FILE" 2>/dev/null || echo "")

  if [[ -z "$CURRENT_JWT" || "$CURRENT_JWT" == "" ]]; then
    JWT_SECRET=$(secret "JWT_SECRET" "JWT Secret (min 32 chars, or press Enter to auto-generate)")
    if [[ -z "$JWT_SECRET" ]]; then
      JWT_SECRET=$(openssl rand -base64 32 2>/dev/null || head -c 32 /dev/urandom | base64)
      log "Generated random JWT_SECRET"
    fi
    sed -i.bak "s|^JWT_SECRET=.*|JWT_SECRET=$JWT_SECRET|" "$ENV_FILE"
    rm -f "${ENV_FILE}.bak"
  else
    JWT_SECRET="$CURRENT_JWT"
    info "JWT_SECRET already set"
  fi

  if [[ -z "$CURRENT_ADMIN_PW" || "$CURRENT_ADMIN_PW" == "" ]]; then
    ADMIN_PASSWORD=$(secret "ADMIN_PASSWORD" "Admin password (or press Enter to auto-generate)")
    if [[ -z "$ADMIN_PASSWORD" ]]; then
      ADMIN_PASSWORD=$(openssl rand -base64 16 2>/dev/null || head -c 16 /dev/urandom | base64)
      log "Generated random ADMIN_PASSWORD"
    fi
    sed -i.bak "s|^ADMIN_PASSWORD=.*|ADMIN_PASSWORD=$ADMIN_PASSWORD|" "$ENV_FILE"
    rm -f "${ENV_FILE}.bak"
  else
    ADMIN_PASSWORD="$CURRENT_ADMIN_PW"
    info "ADMIN_PASSWORD already set"
  fi

  echo ""
  warn "Save these credentials:"
  echo -e "  ${BOLD}Admin username:${NC} admin"
  echo -e "  ${BOLD}Admin password:${NC} $ADMIN_PASSWORD"
  echo -e "  ${BOLD}JWT Secret:${NC}     $JWT_SECRET"
else
  # Dev: sensible defaults, just let user tweak if they want
  DEV_JWT=$(grep -oP '^JWT_SECRET=\K.*' "$ENV_FILE" 2>/dev/null || echo "")
  if [[ -z "$DEV_JWT" || "$DEV_JWT" == "dev-only-change-me-at-least-32-characters" ]]; then
    NEW_JWT=$(prompt "JWT_SECRET" "JWT Secret (press Enter for dev default)" "dev-only-change-me-at-least-32-characters")
    sed -i.bak "s|^JWT_SECRET=.*|JWT_SECRET=$NEW_JWT|" "$ENV_FILE"
    rm -f "${ENV_FILE}.bak"
  fi
  info "Using dev defaults (login: admin / admin123)"
fi

# Optional overrides
echo ""
PORT=$(prompt "PORT" "Server port" "3000")
sed -i.bak "s|^PORT=.*|PORT=$PORT|" "$ENV_FILE" 2>/dev/null || true
rm -f "${ENV_FILE}.bak"

if [[ "$DEPLOY_MODE" == "prod" ]]; then
  info "Other variables can be edited in $ENV_FILE"
fi

log "Environment configured"
echo ""

# ── Step 4: Install & run ──────────────────────────────────────
header "Step 5/5 — Installing and starting services"

if [[ "$RUNTIME" == "docker" ]]; then
  info "Building and starting Docker containers..."
  echo ""

  if [[ "$DEPLOY_MODE" == "prod" ]]; then
    docker compose up -d --build
  else
    docker compose -f docker-compose.dev.yml up -d --build
  fi

  # Health check
  log "Waiting for services to start..."
  HEALTHY=false
  for i in $(seq 1 30); do
    if curl -sf "http://localhost:${PORT}/health" >/dev/null 2>&1; then
      HEALTHY=true
      break
    fi
    sleep 2
    echo -n "."
  done
  echo ""

  if [[ "$HEALTHY" == "true" ]]; then
    log "Backend is healthy!"
  else
    warn "Health check timed out — services may still be starting"
    if [[ "$DEPLOY_MODE" == "dev" ]]; then
      warn "Check logs: docker compose -f docker-compose.dev.yml logs"
    else
      warn "Check logs: docker compose logs"
    fi
  fi

else
  # Bare-metal install
  info "Installing bare-metal dependencies..."
  echo ""

  # Node dependencies
  if [[ -d "client" ]]; then
    log "Installing client dependencies..."
    npm install --workspace client
  fi

  # Go build
  if [[ -d "backend" ]]; then
    log "Building Go backend..."
    cd backend && make build && cd ..
  fi

  # Start services
  log "Starting services..."

  # Start backend in background
  ./backend/bin/netmonitor &
  BACKEND_PID=$!

  log "Backend started (PID: $BACKEND_PID)"
  log "Dashboard: http://localhost:${PORT}"

  # Trap to kill on exit
  trap "kill $BACKEND_PID 2>/dev/null; exit" INT TERM
fi

# ── Done ────────────────────────────────────────────────────────
header "Installation Complete!"

if [[ "$DEPLOY_MODE" == "prod" ]]; then
  echo -e "  ${BOLD}Dashboard:${NC}  http://localhost:${PORT}"
  echo -e "  ${BOLD}Login:${NC}      admin / $ADMIN_PASSWORD"
  echo -e "  ${BOLD}Health:${NC}     http://localhost:${PORT}/health"
else
  echo -e "  ${BOLD}Frontend:${NC}  http://localhost:5173"
  echo -e "  ${BOLD}Backend:${NC}   http://localhost:${PORT}"
  echo -e "  ${BOLD}Login:${NC}     admin / admin123"
fi

echo ""
echo -e "  ${BOLD}Useful commands:${NC}"
if [[ "$RUNTIME" == "docker" ]]; then
  if [[ "$DEPLOY_MODE" == "dev" ]]; then
    echo "    View logs:   docker compose -f docker-compose.dev.yml logs -f"
    echo "    Stop:        docker compose -f docker-compose.dev.yml down"
    echo "    Restart:     docker compose -f docker-compose.dev.yml restart"
  else
    echo "    View logs:   docker compose logs -f"
    echo "    Stop:        docker compose down"
    echo "    Restart:     docker compose restart"
    echo "    Rebuild:     docker compose up -d --build"
  fi
else
  echo "    Stop:        kill $BACKEND_PID"
  echo "    Restart:     ./backend/bin/netmonitor"
fi

echo ""
log "Enjoy Rayavriti NetMonitor!"
