# Deployment & Operations Guide
## Simplified Network Monitoring System

**Version:** 1.0  
**Date:** January 19, 2026

---

## Infrastructure Requirements

### Minimum Requirements (Small Deployment)
- **CPU:** 4 cores
- **RAM:** 8 GB
- **Storage:** 100 GB SSD
- **Network:** 100 Mbps
- **Monitored Devices:** Up to 100 devices

### Recommended Requirements (Medium Deployment)
- **CPU:** 8 cores
- **RAM:** 16 GB
- **Storage:** 500 GB SSD (RAID 10)
- **Network:** 1 Gbps
- **Monitored Devices:** 100-500 devices

### Enterprise Requirements (Large Deployment)
- **CPU:** 16+ cores
- **RAM:** 32+ GB
- **Storage:** 1+ TB SSD (RAID 10)
- **Network:** 10 Gbps
- **Monitored Devices:** 500+ devices

---

## Installation Guide

### Prerequisites

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Install Docker Compose
sudo apt install docker-compose-plugin

# Install Node.js (if running without Docker)
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs

# Install PostgreSQL client tools
sudo apt install -y postgresql-client
```

---

## Docker Deployment

### 1. Clone Repository

```bash
git clone https://github.com/yourorg/network-monitor.git
cd network-monitor
```

### 2. Configure Environment

Create `.env` file:

```bash
# Database Configuration
POSTGRES_USER=monitoring
POSTGRES_PASSWORD=securePassword123
POSTGRES_DB=monitoring
TIMESCALEDB_PASSWORD=securePassword123

# Redis Configuration
REDIS_PASSWORD=redisPassword123

# Application Configuration
NODE_ENV=production
API_PORT=3000
WS_PORT=3001
JWT_SECRET=your-super-secret-jwt-key-change-this
SESSION_SECRET=your-session-secret-change-this

# SMTP Configuration
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=alerts@example.com
SMTP_PASSWORD=yourEmailPassword

# Admin User
ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=adminPassword123
```

### 3. Start Services

```bash
# Build and start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Check service status
docker-compose ps
```

### 4. Initialize Database

```bash
# Run migrations
docker-compose exec api npm run db:migrate

# Seed initial data
docker-compose exec api npm run db:seed
```

### 5. Access Application

- **Web UI:** http://localhost:3000
- **API:** http://localhost:3000/api/v1
- **WebSocket:** ws://localhost:3001

---

## Manual Installation

### 1. Install PostgreSQL + TimescaleDB

```bash
# Add PostgreSQL repository
sudo sh -c 'echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'
wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | sudo apt-key add -

# Install PostgreSQL
sudo apt update
sudo apt install -y postgresql-15

# Add TimescaleDB repository
sudo add-apt-repository ppa:timescale/timescaledb-ppa
sudo apt update

# Install TimescaleDB
sudo apt install -y timescaledb-2-postgresql-15

# Configure TimescaleDB
sudo timescaledb-tune --quiet --yes

# Restart PostgreSQL
sudo systemctl restart postgresql
```

### 2. Install Redis

```bash
sudo apt install -y redis-server

# Configure Redis
sudo nano /etc/redis/redis.conf
# Set: requirepass yourRedisPassword

# Restart Redis
sudo systemctl restart redis
```

### 3. Install Application

```bash
# Clone repository
git clone https://github.com/yourorg/network-monitor.git
cd network-monitor

# Install backend dependencies
cd backend
npm install

# Install frontend dependencies
cd ../frontend
npm install

# Build frontend
npm run build
```

### 4. Configure Application

```bash
# Copy environment template
cp .env.example .env

# Edit configuration
nano .env
```

### 5. Start Services

```bash
# Start API server
cd backend
npm run start:prod

# Or use PM2 for process management
pm2 start npm --name "monitor-api" -- run start:prod
pm2 start npm --name "monitor-ws" -- run start:ws
pm2 start npm --name "monitor-collector" -- run start:collector

# Save PM2 configuration
pm2 save
pm2 startup
```

---

## Configuration

### Database Configuration

**PostgreSQL:**
```sql
-- Create database
CREATE DATABASE monitoring;

-- Create TimescaleDB extension
\c monitoring
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Create user
CREATE USER monitoring WITH PASSWORD 'securePassword';
GRANT ALL PRIVILEGES ON DATABASE monitoring TO monitoring;
```

**Connection String:**
```
postgresql://monitoring:securePassword@localhost:5432/monitoring
```

---

### NGINX Configuration

```nginx
# /etc/nginx/sites-available/monitoring

upstream api_backend {
    server localhost:3000;
}

upstream ws_backend {
    server localhost:3001;
}

server {
    listen 80;
    server_name monitor.example.com;
    
    # Redirect to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name monitor.example.com;
    
    ssl_certificate /etc/ssl/certs/monitor.crt;
    ssl_certificate_key /etc/ssl/private/monitor.key;
    
    # API proxy
    location /api/ {
        proxy_pass http://api_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    
    # WebSocket proxy
    location /socket.io/ {
        proxy_pass http://ws_backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }
    
    # Static files
    location / {
        root /var/www/monitoring/dist;
        try_files $uri $uri/ /index.html;
    }
}
```

---

## Monitoring & Health Checks

### Health Check Endpoints

```bash
# API health
curl http://localhost:3000/health

# Database connection
curl http://localhost:3000/health/database

# Redis connection
curl http://localhost:3000/health/redis
```

### Prometheus Metrics

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'monitoring-api'
    static_configs:
      - targets: ['localhost:3000']
    metrics_path: /metrics
```

---

## Backup & Recovery

### Database Backup

**Automated Backup Script:**
```bash
#!/bin/bash
# /opt/monitoring/backup.sh

BACKUP_DIR="/var/backups/monitoring"
DATE=$(date +%Y%m%d_%H%M%S)

# PostgreSQL backup
pg_dump -U monitoring monitoring | gzip > "$BACKUP_DIR/db_$DATE.sql.gz"

# Remove backups older than 30 days
find $BACKUP_DIR -name "db_*.sql.gz" -mtime +30 -delete
```

**Cron Schedule:**
```bash
# Run daily at 2 AM
0 2 * * * /opt/monitoring/backup.sh
```

### Configuration Backup

```bash
# Backup configuration and dashboards
curl -H "Authorization: Bearer $API_TOKEN" \
  http://localhost:3000/api/v1/export/full > backup_$(date +%Y%m%d).json
```

### Restore Database

```bash
# Restore from backup
gunzip < /var/backups/monitoring/db_20260119.sql.gz | \
  psql -U monitoring monitoring
```

---

## Scaling Strategies

### Horizontal Scaling

**Load Balancer Configuration:**
```nginx
upstream api_cluster {
    least_conn;
    server 10.0.1.101:3000;
    server 10.0.1.102:3000;
    server 10.0.1.103:3000;
}
```

**Collector Distribution:**
```yaml
# Deploy collectors on separate hosts
collector-1:
  monitors: ["192.168.1.0/24"]
  
collector-2:
  monitors: ["192.168.2.0/24"]
  
collector-3:
  monitors: ["192.168.3.0/24"]
```

### Database Scaling

**Read Replicas:**
```bash
# Configure PostgreSQL replication
# On primary server
hot_standby = on
wal_level = replica
max_wal_senders = 3

# On replica server
primary_conninfo = 'host=primary port=5432 user=replication'
```

**TimescaleDB Partitioning:**
```sql
-- Automatic partitioning by time
SELECT create_hypertable('metrics', 'time', 
  chunk_time_interval => INTERVAL '1 day');
```

---

## Security Hardening

### 1. Firewall Configuration

```bash
# UFW configuration
sudo ufw allow 22/tcp   # SSH
sudo ufw allow 80/tcp   # HTTP
sudo ufw allow 443/tcp  # HTTPS
sudo ufw enable
```

### 2. SSL/TLS Configuration

```bash
# Generate Let's Encrypt certificate
sudo certbot --nginx -d monitor.example.com

# Auto-renewal
sudo systemctl enable certbot.timer
```

### 3. Database Security

```sql
-- Restrict PostgreSQL access
# pg_hba.conf
host    monitoring    monitoring    10.0.0.0/8    md5
host    all           all           0.0.0.0/0     reject
```

### 4. Application Security

```bash
# Set secure file permissions
chmod 600 .env
chown monitoring:monitoring .env

# Disable root login
PermitRootLogin no  # /etc/ssh/sshd_config
```

---

## Troubleshooting

### Issue: High Database CPU

**Diagnosis:**
```sql
-- Find slow queries
SELECT pid, query, state, query_start
FROM pg_stat_activity
WHERE state != 'idle'
ORDER BY query_start;
```

**Solution:**
- Add missing indexes
- Optimize query patterns
- Increase shared_buffers

---

### Issue: Memory Exhaustion

**Diagnosis:**
```bash
# Check memory usage
free -h
docker stats

# Check application memory
pm2 monit
```

**Solution:**
- Increase system RAM
- Adjust Node.js heap size: `--max-old-space-size=4096`
- Reduce polling frequency

---

### Issue: WebSocket Disconnections

**Diagnosis:**
```bash
# Check WebSocket logs
docker-compose logs websocket

# Test WebSocket connection
wscat -c ws://localhost:3001
```

**Solution:**
- Increase NGINX timeout: `proxy_read_timeout 3600s;`
- Check Redis connectivity
- Verify firewall rules

---

## Maintenance

### Regular Tasks

**Daily:**
- Review alert logs
- Check disk space
- Monitor system performance

**Weekly:**
- Review security logs
- Update dashboards
- Analyze slow queries

**Monthly:**
- Update dependencies
- Review user access
- Test backups
- Capacity planning

### Update Procedure

```bash
# Backup before update
./backup.sh

# Pull latest code
git pull origin main

# Update dependencies
npm install

# Run migrations
npm run db:migrate

# Restart services
pm2 restart all

# Verify
curl http://localhost:3000/health
```

---

## Performance Tuning

### PostgreSQL Optimization

```sql
-- postgresql.conf
shared_buffers = 4GB
effective_cache_size = 12GB
maintenance_work_mem = 1GB
work_mem = 64MB
max_connections = 200
```

### Node.js Optimization

```bash
# Increase file descriptor limits
ulimit -n 65536

# Enable cluster mode
NODE_ENV=production \
  node --max-old-space-size=4096 \
  cluster.js
```

### Redis Optimization

```conf
# redis.conf
maxmemory 2gb
maxmemory-policy allkeys-lru
```

---

## Monitoring Best Practices

1. **Set Realistic Thresholds**: Avoid alert fatigue
2. **Use Dependencies**: Prevent cascading alerts
3. **Regular Review**: Adjust thresholds based on baselines
4. **Document Changes**: Track configuration changes
5. **Test Alerts**: Verify notification delivery
6. **Capacity Planning**: Monitor growth trends

---

## Support & Resources

### Logs Location
- **Application:** `/var/log/monitoring/`
- **PostgreSQL:** `/var/log/postgresql/`
- **NGINX:** `/var/log/nginx/`

### Useful Commands

```bash
# View application logs
pm2 logs

# Check database connections
psql -U monitoring -c "SELECT * FROM pg_stat_activity;"

# Test SMTP
echo "Test" | mail -s "Test Email" admin@example.com

# Check disk usage
df -h
du -sh /var/lib/postgresql/
```
