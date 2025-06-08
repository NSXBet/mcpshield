# How to Run MCP Shield

MCP Shield has two components:

## Server Component

The server runs standalone and manages MCP servers defined in your config file. It provides a secure proxy with sandboxed execution and access controls.

Run the MCP Shield server using Docker:

```bash
docker run -p 18001:18001 \
    -v "$PWD/mcp.json:/app/mcp.json" \
    -v "$PWD/config.yaml:/app/config.yaml" \
    ghcr.io/nsxbet/mcp-shield:latest
```

## Client Component

Configure the client in your MCP clients (Claude, Cursor, etc.):

```json
{
  "mcpServers": {
    "MCPShield": {
      "command": "npx",
      "args": ["-y", "@nsxbet/mcp-shield@latest"],
      "env": {
        "API_KEY": "ms_fQ0qBtkvH5hwj1ax5sYxqkOagh0hQX",
        "BASE_URL": "http://localhost:18001"
      }
    }
  }
}
```

# MCP Server Configuration

MCP Shield can be configured to run any MCP server. `mcp.yaml` is the configuration file for MCP Shield.

Example:

```yaml
mcp-servers:
  - name: weather
    command: npx
    args:
      - -y
      - mcp-remote
      - https://weather-mcp.descope.sh/sse
  - name: github-docker
    command: docker
    args:
      - run
      - -i
      - --rm
      - -e
      - GITHUB_PERSONAL_ACCESS_TOKEN
      - ghcr.io/github/github-mcp-server
    env:
      GITHUB_PERSONAL_ACCESS_TOKEN: "$GITHUB_PERSONAL_ACCESS_TOKEN"
```