# Neo4j MCP

Official Model Context Protocol (MCP) server for Neo4j available as a Python Wheel.  

> **Note:** The Python Package version does not match that of neo4j-mcp server.  This is intentional.  To see the version of neo4j-mcp, enter `neo4j-mcp -v` after installation.

## Links

- [Documentation](https://neo4j.com/docs/mcp/current/)
- [Discord](https://discord.gg/neo4j)

## Prerequisites

- A running Neo4j database instance; options include [Aura](https://neo4j.com/product/auradb/), [neo4j–desktop](https://neo4j.com/download/) or [self-managed](https://neo4j.com/deployment-center/#gdb-tab).
- APOC plugin installed in the Neo4j instance.
- Any MCP-compatible client (e.g. [VSCode](https://code.visualstudio.com/) with [MCP support](https://code.visualstudio.com/docs/copilot/customization/mcp-servers))

> **⚠️ Known Issue**: Neo4j version **5.26.18** has a bug in APOC that causes the `get-schema` tool to fail. This issue is fixed in version **5.26.19** and above. If you're using 5.26.18, please upgrade to 5.26.19 or later. See [#136](https://github.com/neo4j/mcp/issues/136) for details.

## Startup Checks & Adaptive Operation

The server performs several pre-flight checks at startup to ensure your environment is correctly configured.

**STDIO Mode - Mandatory Requirements**
In STDIO mode, the server verifies the following core requirements. If any of these checks fail (e.g., due to an invalid configuration, incorrect credentials, or a missing APOC installation), the server will not start:

- A valid connection to your Neo4j instance.
- The ability to execute queries.
- The presence of the APOC plugin.

**Optional Requirements**
If an optional dependency is missing, the server will start in an adaptive mode. For instance, if the Graph Data Science (GDS) library is not detected in your Neo4j installation, the server will still launch but will automatically disable all GDS-related tools, such as `list-gds-procedures`. All other tools will remain available.

## Installation

### pip

```bash
pip install -i https://test.pypi.org/simple/ neo4j-mcp

```

### pipx


```bash
pipx install -i https://test.pypi.org/simple/ neo4j-mcp

```


### uv

```bash
uv tool install --index-url https://test.pypi.org/simple/ neo4j-mcp

```

### uvx

Downloads and executes.  You will need to include the neo4j-mcp args as shown below
```bash
uvx -i https://test.pypi.org/simple/  neo4j-mcp --neo4j-uri YOUR_NEO4J_INSTANCE_URI --neo4j-username YOUR_NEO4J_USERNAME --neo4j-password YOUR_NEO4J_PASSWORD
```


## MCP Client Setup Guide

This guide covers how to configure various MCP clients (VSCode, Claude Desktop, etc.) to use the Neo4j MCP server in stdio mode.


### VSCode Configuration

Create or edit `mcp.json` (docs: https://code.visualstudio.com/docs/copilot/customization/mcp-servers):

```json
{
  "servers": {
    "neo4j": {
      "type": "stdio",
      "command": "uvx",
      "args" : ["-i",  "https://test.pypi.org/simple/", "neo4j-mcp" ],
      "env": {
        "NEO4J_URI": "bolt://localhost:7687",
        "NEO4J_USERNAME": "neo4j",
        "NEO4J_PASSWORD": "password",
        "NEO4J_DATABASE": "neo4j",
        "NEO4J_READ_ONLY": "true",
        "NEO4J_TELEMETRY": "false",
        "NEO4J_LOG_LEVEL": "info",
        "NEO4J_LOG_FORMAT": "text",
        "NEO4J_SCHEMA_SAMPLE_SIZE": "100"
      }
    }
  }
}
```

> **Note:** The first three environment variables (NEO4J_URI, NEO4J_USERNAME, NEO4J_PASSWORD) are **required**. The server will fail to start if any of these are missing.

Restart VSCode; open Copilot Chat and ask: "List Neo4j MCP tools" to confirm.

### Claude Desktop Configuration

First, make sure you have Claude for Desktop installed. [You can install the latest version here](https://claude.ai/download).

Open your Claude for Desktop App configuration at:

- (MacOS/Linux) `~/Library/Application Support/Claude/claude_desktop_config.json`
- (Windows) `path_to_your\claude_desktop_config.json`

Create the file if it doesn't exist, then add the `neo4j-mcp` server:


```json
{
  "mcpServers": {
    "neo4j-mcp": {
      "type": "stdio",
      "command": "uvx",
      "args" : ["-i",  "https://test.pypi.org/simple/", "neo4j-mcp" ],
      "env": {
        "NEO4J_URI": "bolt://localhost:7687",
        "NEO4J_USERNAME": "neo4j",
        "NEO4J_PASSWORD": "password",
        "NEO4J_DATABASE": "neo4j",
        "NEO4J_READ_ONLY": "true",
        "NEO4J_TELEMETRY": "false",
        "NEO4J_LOG_LEVEL": "info",
        "NEO4J_LOG_FORMAT": "text",
        "NEO4J_SCHEMA_SAMPLE_SIZE": "100"
      }
    }
  }
}
```

> **Important:**
> - The first three environment variables (NEO4J_URI, NEO4J_USERNAME, NEO4J_PASSWORD) are ** required**. The server will fail to start if any are missing.
> - Neo4j Desktop default  URI: `bolt://localhost:7687`
> - Aura: use the connection string from the Aura console



## Need Help?

- Check the main [README](../README.md) for general information
- See [CONTRIBUTING](../CONTRIBUTING.md) for development and testing
- Open an issue at https://github.com/neo4j/mcp/issues

