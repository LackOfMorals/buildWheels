# Neo4j CLI ALPHA


> __This is an alpha release and is not suitable for production environments__

Neo4j CLI is a CLI that can be used by people and *AI Agents* to interact with a range of Neo4j capabilities.  Initially this is with Aura and executing Cypher against graph database.  More is to come with each evolution of the CLI. 

As `neo4j-cli` is a labs project and is currently at Alpha stage of development, be aware that:

- It is not supported.
- It may contain breaking changes between its own releases and with the official Neo4j MCP server.
- It should be tested before using.
- __DO NOT USE WITH PRODUCTION ENVIRONMENTS__

## Installation

### pip
`pip install neo4j-cli`

### pipx
`pipx install neo4j-cli`

### uv
`uv tool install  neo4j-cli`

###uvx
Downloads and executes neo4j-cli. You will need to include args for whatever you want to do as part of the command

`uvx -i neo4j-cli YOUR_ARGS_HERE`


### Verify the installation:
`neo4j-cli -v`

Should print the installed version.


## Usage
Create Aura API Credentials in your [Account Settings](https://console.neo4j.io/#account), and note down the client ID and secret.

Add these credentials into the CLI with a name of your choosing:

```bash
./neo4j-cli aura credential add --name "Aura API Credentials" --client-id <client-id> --client-secret <client-secret>
```

This will add and set the credential as the default credential for use.

You can then, for example, list your instances in a table format:

```bash
./neo4j-cli aura instance list --format table
```

If you would rather just type `neo4j-cli` then move the neo4j-cli binary into the file path of your computer.  
Windows:

```bash
move neo4j-cli.exe c:\windows\system32
```

Mac:

```bash
sudo mv neo4j-cli /usr/local/bin
```

To see all of the available commands:

```bash
./neo4j-cli
```

Help for each command is accessed by using it without any flags or options. For example, to see help creating an instance:

```bash
./neo4j-cli aura instance create
```

## Querying Neo4j

`neo4j-cli query` runs Cypher against any Neo4j database via the HTTP Query API. Cypher comes from the positional argument or piped stdin.

```bash
./neo4j-cli query 'RETURN 1 AS n'
echo 'MATCH (n) RETURN count(n)' | ./neo4j-cli query
```

Connection settings are resolved with this precedence (highest first): flag → env var → `.env` file (auto-discovered by walking up from cwd) → built-in default.

| Setting  | Flag         | Env var          | Default                 |
| -------- | ------------ | ---------------- | ----------------------- |
| URI      | `--uri`      | `NEO4J_URI`      | `http://localhost:7474` |
| Username | `--username` | `NEO4J_USERNAME` | `neo4j`                 |
| Password | `--password` | `NEO4J_PASSWORD` | prompted on TTY         |
| Database | `--database` | `NEO4J_DATABASE` | `neo4j`                 |

Bolt-family URIs (`bolt://`, `neo4j://`, `neo4j+s://`, …) are auto-rewritten to `http(s)://` and Aura hosts (`*.neo4j.io`) are forced to `https://<host>:443`.

Pass parameters with `--param key=value` (repeatable). Values that parse as JSON are typed; everything else is a string:

```bash
./neo4j-cli query 'MATCH (p:Person {name:$name}) RETURN p' --param name=Alice
./neo4j-cli query 'RETURN $ids' --param 'ids=[1,2,3]'
echo 'MATCH (p:Person {name:$name}) RETURN p' | ./neo4j-cli query --param name=Alice
```

Output is a table by default; pass `--format json` for a stable envelope (`columns`, `rows`, `truncated`, `arrays_truncated`). When stdout is not a terminal (piped or redirected), `--format` defaults to `json`. Applies to both `query` and `:schema`. Large results are capped at 100 rows and arrays inside cells at 100 items — tune with `--max-rows` / `--truncate-arrays-over` (0 = unlimited).

Schema introspection:

```bash
./neo4j-cli query :schema
```

Use `--insecure` to skip TLS verification for self-signed dev setups.

## Agent skills

`neo4j-cli` ships an embedded skill bundle (`SKILL.md` + per-subcommand references) that teaches AI coding agents how to drive the CLI. `skill install` drops that bundle into the supported agents' skill directories so the agent picks it up on next run.

Supported agents:

-   Claude Code
-   Cursor
-   Windsurf
-   Copilot
-   Gemini CLI
-   Cline
-   Codex
-   Pi
-   OpenCode
-   Junie

Install into every detected agent (or pass an agent name to target one):

```bash
./neo4j-cli skill install
./neo4j-cli skill install claude-code
```

List supported agents and per-agent install state:

```bash
./neo4j-cli skill list
```

Check installed bundles for version drift against the running binary:

```bash
./neo4j-cli skill check
```

Remove the installed bundle (idempotent):

```bash
./neo4j-cli skill remove
./neo4j-cli skill remove claude-code
```

The same `skill` subcommand is available on `aura-cli`, scoped to the standalone Aura surface.

Beta features (commands gated by `AuraBetaEnabled`, e.g. `dataapi`, `import`, `deployment`) are not included in the generated bundle — the bundle reflects the default-config command surface.

## Feedback / Issues

Please use [GitHub issues](https://github.com/neo4j-labs/neo4j-cli/issues) to provide feedback and report any issues that you have encountered.

