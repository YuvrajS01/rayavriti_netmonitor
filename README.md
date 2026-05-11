<div align="center">
  <h1>🌐 Rayavriti NetMonitor</h1>
  <p><em>A professional-grade, real-time network traffic visibility and monitoring platform.</em></p>

  ![Node.js](https://img.shields.io/badge/Node.js-v22-green?style=flat-square&logo=node.js)
  ![React](https://img.shields.io/badge/React-v19-blue?style=flat-square&logo=react)
  ![TypeScript](https://img.shields.io/badge/TypeScript-Ready-blue?style=flat-square&logo=typescript)
  ![License](https://img.shields.io/badge/License-Proprietary-red?style=flat-square)
</div>

---

Rayavriti NetMonitor is a professional-grade network traffic visibility platform. It provides a real-time, event-driven Single Page Application (SPA) dashboard for comprehensive monitoring of IT infrastructure, focusing on modern aesthetics and high-performance packet analysis.

## 📑 Table of Contents
- [✨ Features](#-features)
- [🏗️ Tech Stack](#️-tech-stack)
- [📁 Project Structure](#-project-structure)
- [⚙️ Prerequisites](#️-prerequisites)
- [🚀 Installation](#-installation)
- [💻 Development](#-development)
- [📦 Production Build](#-production-build)
- [🤝 Contributing](#-contributing)

---

## ✨ Features

- ⚡ **Real-Time Network Visibility:** Live traffic analytics and system performance monitoring powered by WebSockets.
- 🔍 **Packet Sniffing & Flow Analysis:** Built-in support for real-time packet sniffing, along with NetFlow and sFlow data collection.
- 🎨 **Interactive Dashboards:** Polished, neon-minimalist user interface with clickable chart components and dynamic visual representations.
- 🖥️ **Deep-Dive Device Modals:** Detailed views for individual devices including live performance graphs, resource metrics, and configuration options.
- 🏥 **System Health Monitoring:** Active tracking of system resources, API endpoints, and network connections via Ping and SNMP.
- 🏗️ **Modern Architecture:** A full-stack, event-driven React SPA backed by a robust Node.js server.

---

## 🏗️ Tech Stack

### Frontend
- **Framework:** React 19 with TypeScript, built via Vite
- **Styling:** Tailwind CSS v4 (Neon-Minimalist UI)
- **State Management:** Redux Toolkit
- **Data Visualization:** Recharts
- **Real-Time Communication:** Socket.IO Client

### Backend
- **Runtime & Framework:** Node.js (v22 LTS) with Express and TypeScript
- **Real-Time Engine:** Socket.IO
- **Database:** SQLite (`better-sqlite3`)
- **Network Tools:** 
  - `cap` (Packet sniffing)
  - `ping` (ICMP monitoring)
  - `net-snmp` (SNMP integration)
  - `node-netflowv9` & `node-sflow` (Flow collection)

---

## 📁 Project Structure

This repository is set up as a monorepo using **npm workspaces**:

```text
rayavriti_netmonitor/
├── client/           # React frontend application
├── server/           # Node.js backend server
├── package.json      # Root package file (Workspace configuration)
└── .nvmrc            # Node version definition (v22)
```

---

## ⚙️ Prerequisites

Before you begin, ensure you have met the following requirements:
- **Node.js:** v22 LTS (Recommended, defined in `.nvmrc`)
- **npm:** v9 or newer
- **Packet Sniffing Permissions:** Running the packet sniffer (via `cap`) may require **administrative/root privileges** or specific capture library installations:
  - **Linux/macOS:** `libpcap`
  - **Windows:** `Npcap`

---

## 🚀 Installation

1. **Clone the repository:**
   ```bash
   git clone <repository-url>
   cd rayavriti_netmonitor
   ```

2. **Use the correct Node version:**
   If you use `nvm`, simply run:
   ```bash
   nvm use
   ```

3. **Install dependencies:**
   This command installs dependencies for all workspaces.
   ```bash
   npm install
   ```

---

## 💻 Development

To run the application locally in development mode, start both the backend server and the frontend client.

**1. Start the Backend Server**
```bash
npm run dev:server
```
> **Note:** You may need to run this with `sudo` on Linux/macOS to allow packet capturing (`sudo npm run dev:server`).

**2. Start the Frontend Client**
Open a new terminal window and run:
```bash
npm run dev:client
```

**3. View the App**
Open your browser and navigate to the URL provided by Vite (typically [http://localhost:5173](http://localhost:5173)).

---

## 📦 Production Build

To build and run the application for production deployment:

**1. Build both client and server:**
```bash
npm run build
```
*(This compiles the TypeScript code in the server and builds the optimized React bundle in the client.)*

**2. Start the production server:**
```bash
npm run start
```
> **Note:** As with development, running the production server may require elevated privileges for packet capture.

---

## 🛠️ Scripts Overview

The root `package.json` includes several helpful scripts:

| Command | Description |
|---|---|
| `npm run dev:server` | Starts the backend in watch mode (nodemon + ts-node) |
| `npm run dev:client` | Starts the frontend Vite development server |
| `npm run build` | Builds both the backend and frontend |
| `npm run start` | Starts the compiled backend production server |

---

## 🤝 Contributing

When contributing to this project, please ensure:
1. **Design Integrity:** UI additions must adhere to the established **neon-minimalist** design language.
2. **Architecture Constraints:** New backend services should integrate into the real-time event-driven architecture using WebSockets where appropriate.
3. **Code Quality:** Maintain TypeScript strictness and clean code principles.

---

## 📄 License

**Proprietary** - All rights reserved.
