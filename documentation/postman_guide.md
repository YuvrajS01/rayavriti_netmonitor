# Rayavriti NetMonitor Postman Guide

This guide details how to interact with the Rayavriti NetMonitor APIs using Postman. It covers authentication, API endpoint formats, and example requests to help you test the backend endpoints efficiently.

---

## 1. Setting Up Postman Environment

To make testing easier, create a new **Environment** in Postman (e.g., `NetMonitor Local`) and define the following variables:

- `base_url`: `http://localhost:3000` (or your production server URL)
- `token`: (Leave blank, will be filled after login)
- `api_key`: (If you are using API key authentication for V1 endpoints)

### Base URL Note
The APIs are generally available at the base URL (e.g., `http://localhost:3000`). Make sure your server is running before making requests.

---

## 2. Authentication Methods

The NetMonitor API supports two main forms of authentication:

1. **JWT Session Tokens (Legacy & V1):** Returned upon successful login. Must be included in the headers.
2. **API Keys (V1 Endpoints only):** Used for system-to-system integration without user login.

### Passing the JWT Token
For authenticated endpoints, you must provide the token in the request headers using one of two methods:
- **Authorization:** `Bearer {{token}}`
- **x-session-token:** `{{token}}`

*In Postman, it is easiest to go to the "Authorization" tab, select "Bearer Token", and enter `{{token}}`.*

### Passing the API Key (V1 Endpoints)
For `/api/v1/*` endpoints, you can authenticate using an API key by setting the header:
- **x-api-key:** `{{api_key}}`

---

## 3. Auth Endpoints

### 3.1 Login (Legacy)
- **Method:** `POST`
- **URL:** `{{base_url}}/api/auth/login`
- **Headers:** `Content-Type: application/json`
- **Body:**
  ```json
  {
    "username": "admin",
    "password": "your_password"
  }
  ```
- **Postman Tip:** After a successful login, copy the `token` from the response `data.token` and update your `{{token}}` environment variable.

### 3.2 Get Current User
- **Method:** `GET`
- **URL:** `{{base_url}}/api/auth/me`
- **Auth:** Bearer Token `{{token}}`

### 3.3 Logout
- **Method:** `POST`
- **URL:** `{{base_url}}/api/auth/logout`
- **Auth:** Bearer Token `{{token}}`

---

## 4. Device Management

### 4.1 Get All Devices
- **Method:** `GET`
- **URL:** `{{base_url}}/api/devices`
- **Auth:** Bearer Token `{{token}}`

### 4.2 Add New Device
- **Method:** `POST`
- **URL:** `{{base_url}}/api/devices`
- **Auth:** Bearer Token `{{token}}`
- **Body:**
  ```json
  {
    "name": "Router Core",
    "host": "192.168.1.1",
    "protocol": "ping"
  }
  ```

### 4.3 Update Device
- **Method:** `PUT`
- **URL:** `{{base_url}}/api/devices/:id` (Replace `:id` with the actual device ID)
- **Auth:** Bearer Token `{{token}}`
- **Body:**
  ```json
  {
    "name": "Router Core Updated",
    "protocol": "snmp",
    "snmpCommunity": "public"
  }
  ```

### 4.4 Delete Device
- **Method:** `DELETE`
- **URL:** `{{base_url}}/api/devices/:id`
- **Auth:** Bearer Token `{{token}}`

### 4.5 Scan Device Ports
- **Method:** `POST`
- **URL:** `{{base_url}}/api/devices/:id/scan-ports`
- **Auth:** Bearer Token `{{token}}`
- **Body:**
  ```json
  {
    "ports": [80, 443, 22],
    "timeoutMs": 1000,
    "concurrency": 4
  }
  ```
- **Note:** All fields in the body are optional. If omitted, default values are used.

---

## 5. Metrics & Telemetry

### 5.1 Get Latest Metrics (All Devices)
- **Method:** `GET`
- **URL:** `{{base_url}}/api/metrics/latest`
- **Auth:** Bearer Token `{{token}}`

### 5.2 Get Metrics for Specific Device
- **Method:** `GET`
- **URL:** `{{base_url}}/api/metrics/device/:id`
- **Auth:** Bearer Token `{{token}}`
- **Query Params:**
  - `limit`: (Optional) Limit the number of records returned (default: 100).

### 5.3 Get Device Insights/Anomalies
- **Method:** `GET`
- **URL:** `{{base_url}}/api/insights`
- **Auth:** Bearer Token `{{token}}`

---

## 6. Alerts Management

### 6.1 Get Active Alerts
- **Method:** `GET`
- **URL:** `{{base_url}}/api/alerts`
- **Auth:** Bearer Token `{{token}}`
- **Query Params:**
  - `status`: (Optional) Defaults to 'active'.
  - `limit`: (Optional) Defaults to 200.

### 6.2 Acknowledge Alert
- **Method:** `POST`
- **URL:** `{{base_url}}/api/alerts/:id/acknowledge`
- **Auth:** Bearer Token `{{token}}`
- **Body:**
  ```json
  {
    "comment": "Investigating network drop"
  }
  ```

### 6.3 Resolve Alert
- **Method:** `POST`
- **URL:** `{{base_url}}/api/alerts/:id/resolve`
- **Auth:** Bearer Token `{{token}}`

---

## 7. Reports & Statistics

### 7.1 Get System Stats Overview
- **Method:** `GET`
- **URL:** `{{base_url}}/api/stats`
- **Auth:** Bearer Token `{{token}}`

### 7.2 Get Reports Summary
- **Method:** `GET`
- **URL:** `{{base_url}}/api/reports/summary`
- **Auth:** Bearer Token `{{token}}`
- **Query Params:**
  - `from`: (Optional) ISO Timestamp
  - `to`: (Optional) ISO Timestamp
  - `deviceId`: (Optional) Filter by specific device

### 7.3 Export Metrics as CSV
- **Method:** `GET`
- **URL:** `{{base_url}}/api/reports/metrics.csv`
- **Auth:** Bearer Token `{{token}}`
- **Note:** You can use Postman's "Save Response" feature to download the resulting CSV file.

### 7.4 Get Report Timeseries
- **Method:** `GET`
- **URL:** `{{base_url}}/api/reports/timeseries`
- **Auth:** Bearer Token `{{token}}`
- **Query Params:**
  - `bucketMinutes`: (Optional) Grouping interval in minutes (default 30).

---

## 8. V1 Endpoints (Rate Limited & API Key Supported)

The application also features a standardized `/api/v1` structure with better rate limiting, paginated responses, and API Key support. These endpoints are great for automation and integration.

**V1 Headers:**
You can authenticate using your normal `Bearer {{token}}` OR by sending `x-api-key: {{api_key}}`.

### 8.1 List Devices (V1)
- **Method:** `GET`
- **URL:** `{{base_url}}/api/v1/devices`
- **Query Params:**
  - `page`: Page number
  - `pageSize`: Items per page
  - `filter[status]`: e.g., 'active', 'down'
  - `sort`: e.g., '-created_at', 'name'

### 8.2 Get Single Device (V1)
- **Method:** `GET`
- **URL:** `{{base_url}}/api/v1/devices/:id`

### 8.3 List Sensors (V1)
- **Method:** `GET`
- **URL:** `{{base_url}}/api/v1/sensors`
- **Query Params:**
  - `deviceId`: Filter sensors by a specific device

### 8.4 Metrics Query (V1)
- **Method:** `GET`
- **URL:** `{{base_url}}/api/v1/metrics/query`
- **Query Params:**
  - `deviceId`: (Required) Device ID
  - `from`: (Required) ISO Timestamp
  - `to`: (Required) ISO Timestamp
  - `aggregation`: 'avg' (default)
  - `interval`: '5m' (default)

---

## Postman Testing Tips

1. **Test Scripts:** You can use Postman's "Tests" tab on the Login request to automatically set your token variable:
   ```javascript
   const res = pm.response.json();
   if (res.data && res.data.token) {
       pm.environment.set("token", res.data.token);
   }
   ```
2. **WebSockets:** While Postman mostly tests REST APIs, you can also use Postman's new WebSocket capabilities to connect to `ws://localhost:3000` to observe real-time telemetry and alerts emitted by `SocketService`.
