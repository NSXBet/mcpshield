# MCP Shield

Security proxy for Model Context Protocol (MCP) servers. Runs MCP servers in containers and provides a unified HTTP endpoint.

## What it does

- Spawns MCP servers from config
- Proxies MCP requests through HTTP
- Runs servers in Kubernetes containers for isolation
- Prefixes tools with `ms_servername_` for routing

## Running

```bash
task run
```

This starts the proxy server on `http://localhost:8080/mcp`

## Configuration

Edit `config.yaml` to define which MCP servers to run:

```yaml
runtime:
  kubernetes:
    namespace: default

mcp-servers:
  - name: github-npx
    image: node:18-alpine
    command: npx
    args:
      - -y
      - "@modelcontextprotocol/server-github"
    env:
      GITHUB_PERSONAL_ACCESS_TOKEN: "$GITHUB_PERSONAL_ACCESS_TOKEN"
```

## Using in Cursor

Add to your Cursor MCP settings:

```json
"mcp-shield": {
  "type": "streamable-http",
  "url": "http://localhost:8080/mcp"
}
```

## Current Phase

Basic MCP server proxying - spawns configured servers and forwards requests with tool prefixing.
