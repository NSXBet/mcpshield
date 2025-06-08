# MCP Shield

Security middleware for Model Context Protocol (MCP) that controls tool access and manages server lifecycles.

## Features

- **Tool Control**: Map, allow/block, and audit MCP tool usage
- **Access Management**: Role-based permissions and policy enforcement  
- **Security**: Parameter validation, rate limiting, and sandboxed execution with limited resources
- **Updates**: Monitor, notify, and manage MCP server version upgrades
- **Monitoring**: Real-time logging and usage analytics

## How It Works

MCP Shield has two components:

### Server Component
Runs standalone and manages the actual MCP servers. It:
- Runs a web server that proxies MCP requests
- Has its own MCP config file declaring which MCP servers to proxy
- Can be configured with an API key that clients must use to connect
- Executes MCP servers in a sandbox environment with limited resources and restricted filesystem access for enhanced security

### Client Component
Runs in your MCP clients (Claude, Cursor, etc.) and connects to the server:
- Connects to the server using BASE_URL and API_KEY
- The client configurations are what you put in your MCP client settings

### Request Flow
MCP clients → Client Component → Server Component → Actual MCP Servers

The server acts as a transparent proxy that intercepts MCP requests, validates against security policies, and controls execution based on configured rules.

See [docs/how-to-run.md](docs/how-to-run.md) for detailed setup instructions.

# Roadmap

## Phase 1: Basic MCP Server Proxying

### Core Functionality
- **Server Component**: Spawn and manage MCP servers from configuration file
- **Client Component**: Connect Cursor (and other MCP clients) to the server
- **Tool Prefixing**: All proxied tools appear with `MS_` prefix in client applications

### Implementation Details
- Server reads MCP server configurations from config file
- Server spawns configured MCP servers as child processes
- Client component connects to server via HTTP/WebSocket
- Server forwards MCP requests/responses between client and spawned servers
- Tool names are prefixed with `MS_` to identify them as proxied through MCP Shield

### Success Criteria
- ✅ Server can read MCP config file
- ✅ Server can spawn MCP servers (filesystem, github, etc.)
- ✅ Server can see all tools available to the client
- ✅ Server can provide new tools to the client with the prefix `MS_`
- ✅ Client component integrates with Cursor
- ✅ Tools appear in Cursor with `MS_` prefix