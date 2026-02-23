# neo4j-mcp Python Wheels Builder

Builds platform-specific Python wheels for the [Neo4j official MCP Server](https://github.com/neo4j/mcp) by downloading pre-built Go binaries from GitHub Releases and packaging them into installable `.whl` files.

Once published to PyPI, users can install the MCP server with no Go toolchain required:

```bash
# pip
pip install neo4j-mcp

# pipx (isolated install, binary on PATH)
pipx install neo4j-mcp

# uv (recommended)
uv tool install neo4j-mcp

# uvx — run without installing
uvx neo4j-mcp
```

---

## How it works

1. Fetches the latest (or a specified) release from `github.com/neo4j/mcp`
2. Downloads the pre-built binary archive for each target platform
3. Extracts the `neo4j-mcp` binary and packages it into a correctly-tagged Python wheel
4. Optionally uploads each wheel to PyPI

The wheel contains a thin Python shim that locates and `exec`s the bundled binary, so `neo4j-mcp` is available on the user's `PATH` after install with no runtime overhead.

---

## Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- A [PyPI API token](https://pypi.org/manage/account/token/) (upload only)
- A GitHub personal access token (optional, avoids API rate limits)

> **Note:** No Python installation is required to build the wheels. Python (or `uv`) is only needed by end users installing the published package.

---

## Quick start

```bash
# Clone this repo
git clone https://github.com/your-org/neo4j-mcp-wheels
cd neo4j-mcp-wheels

# Build wheels for all platforms using the latest neo4j/mcp release
go run build_wheels.go

# Wheels are written to ./dist/
ls dist/
```

---

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-version` | *(latest)* | MCP server release tag to download, e.g. `v1.4.2` |
| `-py-version` | *(mirrors `-version`)* | Python package version, e.g. `1.4.2.1` — useful to re-publish a fixed wheel without a new binary release |
| `-output` | `./dist` | Directory to write `.whl` files into |
| `-platforms` | *(all)* | Comma-separated list of platform keys to build |
| `-upload` | `false` | Upload built wheels to PyPI after building |
| `-pypi-url` | `https://upload.pypi.org/legacy/` | PyPI upload endpoint |
| `-pypi-user` | `__token__` | PyPI username — keep as `__token__` when using an API token |
| `-license` | *(fetched from repo)* | Path to a local license file; defaults to fetching `LICENSE.txt` from `neo4j/mcp` |
| `-description` | `DESCRIPTION.md` | Path to a local Markdown file used as the PyPI long description |
| `-cache` | *(OS cache dir)* | Directory to cache downloaded binaries. Defaults to `~/.cache/neo4j-mcp-wheels` (Linux/macOS) or `%LocalAppData%\neo4j-mcp-wheels` (Windows). Set to `""` to disable |

---

## Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `PYPI_TOKEN` | When `-upload` is set | PyPI API token (starts with `pypi-`) |
| `PYPI_PASSWORD` | When `-upload` is set | Alternative to `PYPI_TOKEN` for password auth |
| `GITHUB_TOKEN` | No | GitHub PAT to avoid API rate limits (60 req/hr unauthenticated vs 5,000 authenticated) |

---

## Supported platforms

| Platform key | Binary | Wheel tag |
|---|---|---|
| `linux_amd64` | `neo4j-mcp` | `manylinux_2_17_x86_64` |
| `linux_arm64` | `neo4j-mcp` | `manylinux_2_17_aarch64` |
| `darwin_amd64` | `neo4j-mcp` | `macosx_10_9_x86_64` |
| `darwin_arm64` | `neo4j-mcp` | `macosx_11_0_arm64` |
| `windows_amd64` | `neo4j-mcp.exe` | `win_amd64` |
| `windows_arm64` | `neo4j-mcp.exe` | `win_arm64` |

---

## Usage examples

### Build wheels for all platforms (latest release)

```bash
go run build_wheels.go
```

### Build wheels for a specific release

```bash
go run build_wheels.go -version v1.4.0
```

### Build for specific platforms only

```bash
go run build_wheels.go -platforms linux_amd64,darwin_arm64
```

### Build and upload to PyPI

```bash
PYPI_TOKEN=pypi-xxxxxxxxxxxx go run build_wheels.go -upload
```

### Test against TestPyPI first (recommended)

```bash
PYPI_TOKEN=pypi-xxxxxxxxxxxx go run build_wheels.go \
  -upload \
  -pypi-url https://test.pypi.org/legacy/
```

Then verify the install from TestPyPI before publishing to production:

```bash
# pip
pip install --index-url https://test.pypi.org/simple/ neo4j-mcp

# uv
uv tool install --index-url https://test.pypi.org/simple/ neo4j-mcp

neo4j-mcp -v
```

### Compile for repeated use

```bash
go build -o build_wheels
./build_wheels -upload
```

---

## GitHub Actions

The included workflow (`.github/workflows/release.yml`) automates the full pipeline. It can be triggered manually, on a schedule, or via a `repository_dispatch` event sent from `neo4j/mcp` after a new release.


- check-version — runs first for all trigger types. It resolves which tag to use (from input, webhook payload, or GitHub API), then does a quick PyPI API check to see if that version is already published. If it is, all downstream jobs are skipped — so the daily schedule doesn't re-publish the same version over and over.
build — runs build_wheels.go and uploads the wheels as a GitHub Actions artifact. It also hooks into actions/cache using the binary version as the cache key, so repeated runs for the same version don't re-download from GitHub. Separating build from publish means you can inspect the wheels before they go to PyPI.
publish — downloads the artifact and pushes to PyPI. The if: condition controls when publishing actually fires:

- workflow_dispatch — only publishes if you explicitly chose upload: true
repository_dispatch / schedule — always publishes (these are only triggered by real release events)

It's configured for OIDC trusted publishing by default (no stored secret needed), but the API token fallback is one uncommented line away.
notify — opens a GitHub issue automatically if any job fails. Useful for the scheduled/webhook triggers where there's nobody watching the run.

### Setup checklist:

PyPI trusted publishing — go to your PyPI project → Publishing → add a GitHub publisher with owner, repo, and workflow name release.yml
Webhook from neo4j/mcp (optional but recommended over polling) — add this to their release workflow, or create a GitHub App that listens for release events:

yaml- name: Trigger wheels build
  run: |
    curl -X POST \
      -H "Authorization: Bearer ${{ secrets.WHEELS_REPO_PAT }}" \
      -H "Accept: application/vnd.github+json" \
      https://api.github.com/repos/your-org/neo4j-mcp-wheels/dispatches \
      -d '{"event_type":"new-mcp-release","client_payload":{"tag":"${{ github.ref_name }}"}}'

Environment protection (optional) — create a pypi environment in Settings → Environments and add required reviewers if you want a manual approval gate before publishing

### Setup

1. Go to **Settings → Secrets and variables → Actions** in your repository
2. Add a secret named `PYPI_TOKEN` with your PyPI API token

### Trigger manually

From the **Actions** tab, select **Build and Publish Wheels**, click **Run workflow**, and optionally enter a specific release tag. Leave the tag blank to use the latest release.

### Trigger on a schedule

The workflow runs daily at 09:00 UTC by default. Edit the `cron` expression in `.github/workflows/release.yml` to change this.

### Trigger via webhook (recommended)

To publish immediately when `neo4j/mcp` cuts a new release, configure a `repository_dispatch` in that repo's release workflow:

```yaml
- name: Notify wheels repo
  run: |
    curl -X POST \
      -H "Authorization: Bearer ${{ secrets.WHEELS_REPO_TOKEN }}" \
      -H "Accept: application/vnd.github+json" \
      https://api.github.com/repos/your-org/neo4j-mcp-wheels/dispatches \
      -d '{"event_type":"new-release","client_payload":{"tag":"${{ github.ref_name }}"}}'
```

---

## PyPI setup

Before the first upload you need to create the package on PyPI and configure trusted publishing (recommended) or a scoped API token.

### Option A — Trusted publishing (OIDC, no stored secrets)

1. Log in to [pypi.org](https://pypi.org) and go to **Your projects → Add project**
2. Under **Publishing**, add a **GitHub Actions** trusted publisher:
   - Owner: `your-org`
   - Repository: `neo4j-mcp-wheels`
   - Workflow: `release.yml`
3. In `.github/workflows/release.yml` ensure the job has `id-token: write` permission (already included)
4. Remove the `PYPI_TOKEN` secret — OIDC handles auth automatically

### Option B — API token

1. Log in to [pypi.org](https://pypi.org) and go to **Account settings → API tokens**
2. Create a token scoped to the `neo4j-mcp` project
3. Add it as `PYPI_TOKEN` in your repository secrets

---

## Troubleshooting

**Asset not found**

The script prints a `[SKIP]` message with the expected asset name. Check the [neo4j/mcp releases page](https://github.com/neo4j/mcp/releases) to confirm the actual archive filenames and update the `assetName` format in `build_wheels.go` if needed.

**GitHub rate limit (403)**

Set the `GITHUB_TOKEN` environment variable with a personal access token to raise the limit from 60 to 5,000 requests per hour.

**PyPI 400 — File already exists**

This is non-fatal. The script logs a warning and continues. PyPI does not allow overwriting an existing release; bump the version in the source repo instead.

**Wheel installs but binary does not run**

Verify the executable bit is set correctly — the shim relies on `os.execv` (Unix) or `subprocess` (Windows) to hand off to the bundled binary. Re-running the build and reinstalling usually resolves this.
