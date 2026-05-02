# API Documentation
## Simplified Network Monitoring System API v1

**Base URL:** `https://api.example.com/api/v1`  
**Version:** 1.0  
**Date:** January 19, 2026

---

## Authentication

### JWT Bearer Token

**Login:**
```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "securePass123"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "accessToken": "eyJhbGc...",
    "refreshToken": "eyJhbGc...",
    "expiresIn": 900,
    "user": {
      "id": "uuid",
      "username": "admin",
      "role": "administrator"
    }
  }
}
```

**Using Token:**
```http
Authorization: Bearer eyJhbGc...
```

### API Key
```http
X-API-Key: sk_live_abc123
```

---

## Response Format

```typescript
{
  "success": boolean,
  "data": T | null,
  "error": {
    "code": string,
    "message": string
  } | null,
  "meta": {
    "timestamp": string,
    "requestId": string,
    "pagination": {...} | null
  }
}
```

## Pagination & Filtering

```http
GET /devices?page=2&pageSize=20&filter[status]=active&sort=-created_at
```

---

## Devices API

### GET /devices
List all devices.

**Response:**
```json
{
  "data": [
    {
      "id": "uuid",
      "name": "Server-01",
      "type": "server",
      "ipAddress": "192.168.1.100",
      "status": "active",
      "sensorCount": 12
    }
  ]
}
```

### POST /devices
Create device.

**Request:**
```json
{
  "name": "Server-02",
  "type": "server",
  "ipAddress": "192.168.1.101"
}
```

---

## Sensors API

### POST /sensors
Create sensor.

**Request:**
```json
{
  "deviceId": "uuid",
  "name": "CPU Usage",
  "type": "snmp",
  "interval": 60,
  "config": {
    "oid": "1.3.6.1.4.1.2021.11.9.0"
  }
}
```

---

## Metrics API

### GET /metrics/query
Query time-series data.

**Parameters:**
- `deviceId` (required)
- `from` (ISO8601, required)
- `to` (ISO8601, required)
- `aggregation`: avg, min, max
- `interval`: 1m, 5m, 1h, 1d

**Response:**
```json
{
  "data": {
    "series": [
      {
        "sensorName": "CPU Usage",
        "dataPoints": [
          {"timestamp": "2026-01-19T00:00:00Z", "value": 42.5}
        ]
      }
    ]
  }
}
```

---

## Alerts API

### GET /alerts
List alerts.

**Response:**
```json
{
  "data": [
    {
      "id": "uuid",
      "severity": "warning",
      "status": "triggered",
      "message": "CPU usage 85%",
      "deviceName": "Server-01",
      "triggeredAt": "2026-01-19T11:30:00Z"
    }
  ]
}
```

### POST /alerts/:id/acknowledge
```json
{"comment": "Investigating"}
```

---

## Dashboards API

### POST /dashboards
Create dashboard.

**Request:**
```json
{
  "name": "Network Overview",
  "widgets": [
    {
      "type": "chart",
      "config": {"sensorId": "uuid"},
      "position": {"x": 0, "y": 0, "w": 6, "h": 3}
    }
  ]
}
```

---

## WebSocket API

### Connection
```javascript
io('wss://api.example.com', {
  auth: { token: 'jwt...' }
});
```

### Events

**Client → Server:**
```javascript
socket.emit('subscribe:device', 'device-uuid');
```

**Server → Client:**
```javascript
socket.on('metric:update', (data) => {
  // Real-time metric updates
});

socket.on('alert:triggered', (alert) => {
  // New alerts
});
```

---

## Error Codes

| Code | Status | Description |
|------|--------|-------------|
| `UNAUTHORIZED` | 401 | Auth required |
| `FORBIDDEN` | 403 | No permission |
| `RESOURCE_NOT_FOUND` | 404 | Not found |
| `VALIDATION_ERROR` | 400 | Invalid input |
| `RATE_LIMIT_EXCEEDED` | 429 | Too many requests |

---

## Rate Limits

- **Authenticated**: 1000 req/hour
- **API Keys**: 5000 req/hour
- **WebSocket**: 10 concurrent connections

**Headers:**
```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1642598400
```
