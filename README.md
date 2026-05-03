# Rayavriti NetMonitor

Rayavriti NetMonitor is a professional-grade network traffic visibility platform. It provides a real-time, event-driven Single Page Application (SPA) dashboard for comprehensive monitoring of IT infrastructure, focusing on modern aesthetics and high-performance packet analysis.

## Features

- **Real-Time Network Visibility:** Live traffic analytics and system performance monitoring powered by WebSockets.
- **Packet Sniffing & Flow Analysis:** Built-in support for real-time packet sniffing, along with NetFlow and sFlow data collection.
- **Interactive Dashboards:** Polished, neon-minimalist user interface with clickable chart components and dynamic visual representations.
- **Deep-Dive Device Modals:** Detailed views for individual devices including live performance graphs, resource metrics, and configuration options.
- **System Health Monitoring:** Active tracking of system resources, API endpoints, and network connections via Ping and SNMP.
- **Modern Architecture:** A full-stack, event-driven React SPA backed by a robust Node.js server.

## Tech Stack

### Frontend
- **Framework:** React 19 with TypeScript, built via Vite
- **Styling:** Tailwind CSS v4 for a neon-minimalist UI
- **State Management:** Redux Toolkit
- **Data Visualization:** Recharts
- **Real-Time Communication:** Socket.IO Client

### Backend
- **Runtime & Framework:** Node.js with Express and TypeScript
- **Real-Time Engine:** Socket.IO
- **Database:** SQLite (`better-sqlite3`)
- **Network Tools:** 
  - `cap` (packet sniffing)
  - `ping` (ICMP monitoring)
  - `net-snmp` (SNMP integration)
  - `node-netflowv9` & `node-sflow` (Flow collection)

## Project Structure

This repository is set up as a monorepo using npm workspaces:

- `/client` - Contains the React frontend application.
- `/server` - Contains the Node.js backend server.

## Prerequisites

- **Node.js:** v18 or newer
- **npm:** v9 or newer
- **Packet Sniffing Permissions:** Depending on your operating system, running the packet sniffer (using `cap`) may require administrative or root privileges, or specific capture library installations (e.g., `libpcap` on Linux/macOS, or `Npcap` on Windows).

## Installation

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd "Rayavriti NetMonitor"
   ```

2. Install dependencies for all workspaces:
   ```bash
   npm install
   ```

## Running for Development

To run the application locally in development mode, you will need to start both the backend server and the frontend client.

1. **Start the Backend Server:**
   ```bash
   npm run dev:server
   ```
   *(Note: You may need to run this with `sudo` on Linux/macOS to allow packet capturing).*

2. **Start the Frontend Client:**
   Open a new terminal window and run:
   ```bash
   npm run dev:client
   ```

3. Open your browser and navigate to the URL provided by Vite (typically `http://localhost:5173`).

## Building for Production

To build the application for production deployment:

1. Build both client and server:
   ```bash
   npm run build
   ```
   This will compile the TypeScript code in the server and build the optimized React bundle in the client.

2. Start the production server:
   ```bash
   npm run start
   ```

## Scripts Overview (Root)

The root `package.json` includes several helpful scripts:

- `npm run dev:server`: Starts the backend in watch mode (nodemon + ts-node).
- `npm run dev:client`: Starts the frontend Vite development server.
- `npm run build`: Builds both the backend and frontend.
- `npm run start`: Starts the compiled backend production server.

## Contributing
When contributing to this project, please ensure that any UI additions adhere to the established neon-minimalist design language, and that new backend services are integrated into the real-time event-driven architecture using WebSockets where appropriate.

## License
Proprietary
