# buildwheels

A generic Go utility that fetches pre-built binary archives from any GitHub Release and packages each one as a platform-specific Python wheel (`.whl`).

Once the wheels are published to PyPI, users can install any Go-distributed binary with no Go toolchain required:

```bash
# pip
pip install <package>

# pipx (isolated install, binary on PATH)
pipx install <package>

# uv (recommended)
uv tool install <package>

# uvx — download and run without a permanent install
uvx <package>
```

---

## How it works

1. Fetches the latest (or a specified) release from any GitHub repository
2. Resolves which binary archives to download — automatically from the release metadata, or from an explicit list you provide
3. Downloads each archive (with optional local caching) and extracts the binary
4. Builds a correctly-tagged Python wheel containing the binary and a thin launcher shim
5. Optionally uploads each wheel to a PyPI-compatible index

The wheel contains a thin Python shim that locates and `exec`s the bundled binary, so the command is available on the user's `PATH` after install with no runtime overhead.

---

## Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- A [PyPI API token](https://pypi.org/manage/account/token/) (upload only)
- A GitHub personal access token (optional, avoids API rate limits)

> **Note:** No Python installation is required to *build* the wheels. Python (or `uv`) is only needed by end users installing the published package.

---

## Quick start

```bash
git clone https://github.com/your-org/buildwheels
cd buildwheels

# Build wheels for all platforms using the latest release of owner/myrepo
go run . -repo owner/myrepo

# Wheels are written to ./dist/
ls dist/
```

---

## Flags

### Required

| Flag | Description |
|------|-------------|
| `-repo` | GitHub repository in `owner/name` format, e.g. `neo4j/mcp` |

### Package identity

These all default to a value derived from the repository name when omitted.

| Flag | Default | Description |
|------|---------|-------------|
| `-binary-name` | repo name | Filename of the binary inside the downloaded archives |
| `-package-name` | binary-name | Python package name published to PyPI |
| `-entry-point` | binary-name | `console_scripts` entry point registered in the wheel |
| `-summary` | derived | One-line description shown on PyPI |
| `-license-expr` | `MIT` | [SPDX licence expression](https://spdx.org/licenses/) embedded in wheel metadata |

### Build

| Flag | Default | Description |
|------|---------|-------------|
| `-version` | *(latest)* | Release tag to download, e.g. `v1.4.2` |
| `-py-version` | *(mirrors `-version`)* | Python package version — useful to re-publish a fixed wheel without a new binary release, e.g. `1.4.2.1` |
| `-output` | `./dist` | Directory to write `.whl` files into |
| `-platforms` | *(all)* | Comma-separated GoReleaser OS_Arch keys to build, e.g. `Linux_x86_64,Darwin_arm64` |
| `-assets` | *(auto-detect)* | Comma-separated asset filenames to download, overriding automatic platform detection |

### PyPI upload

| Flag | Default | Description |
|------|---------|-------------|
| `-upload` | `false` | Upload built wheels to PyPI after building |
| `-pypi-url` | `https://upload.pypi.org/legacy/` | PyPI upload endpoint |
| `-pypi-user` | `__token__` | PyPI username — keep as `__token__` when using an API token |

### Input files

| Flag | Default | Description |
|------|---------|-------------|
| `-license` | *(fetched from repo)* | Path to a local licence file. When omitted, `LICENSE.txt` then `LICENSE` are fetched from the main branch of `-repo` |
| `-description` | `DESCRIPTION.md` | Path to a local Markdown file used as the PyPI long description |

### Logging and caching

| Flag | Default | Description |
|------|---------|-------------|
| `-cache` | *(OS cache dir)* | Directory to cache downloaded binaries. Defaults to `~/.cache/buildwheels` (Linux/macOS) or `%LocalAppData%\buildwheels` (Windows). Set to `""` to disable |
| `-debug` | `false` | Enable debug-level log output (all log lines go to stderr) |

---

## Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `PYPI_TOKEN` | When `-upload` is set | PyPI API token (starts with `pypi-`) |
| `PYPI_PASSWORD` | When `-upload` is set | Alternative to `PYPI_TOKEN` |
| `GITHUB_TOKEN` | No | GitHub PAT; raises rate limit from 60 to 5,000 requests per hour |

---

## Supported platforms

Asset auto-detection matches the [GoReleaser](https://goreleaser.com) default archive naming convention:
`{binary}_{version}_{OS_Arch}.{ext}` with a no-version fallback of `{binary}_{OS_Arch}.{ext}`.

| Platform key (`-platforms`) | Archive | Binary in archive | Wheel tag |
|---|---|---|---|
| `Linux_x86_64` | `.tar.gz` | `<binary>` | `manylinux_2_17_x86_64` |
| `Linux_arm64` | `.tar.gz` | `<binary>` | `manylinux_2_17_aarch64` |
| `Darwin_x86_64` | `.tar.gz` | `<binary>` | `macosx_10_9_x86_64` |
| `Darwin_arm64` | `.tar.gz` | `<binary>` | `macosx_11_0_arm64` |
| `Windows_x86_64` | `.zip` | `<binary>.exe` | `win_amd64` |
| `Windows_arm64` | `.zip` | `<binary>.exe` | `win_arm64` |

---

## Usage examples

### Build wheels for all platforms (latest release)

The `-binary-name` flag must match the filename of the binary **inside the downloaded archives** (and the prefix of the asset filenames on the releases page). When the binary name differs from the repository name — as is the case for `neo4j/mcp` where archives are named `neo4j-mcp_*.tar.gz` — you must supply it explicitly:

```bash
go run . -repo neo4j/mcp -binary-name neo4j-mcp
```

When the binary name matches the repository name (the common case), you can omit the flag:

```bash
go run . -repo goreleaser/goreleaser
```

### Build for a specific release tag

```bash
go run . -repo neo4j/mcp -binary-name neo4j-mcp -version v1.4.0
```

### Override the Python package version

Useful to re-publish a corrected wheel without a new upstream binary release:

```bash
go run . -repo neo4j/mcp -binary-name neo4j-mcp -version v1.4.0 -py-version 1.4.0.1
```

### Build for specific platforms only

Platform keys are case-sensitive and must match the OS_Arch component of the asset filenames exactly:

```bash
go run . -repo neo4j/mcp -binary-name neo4j-mcp -platforms Linux_x86_64,Darwin_arm64
```

### Supply asset names explicitly (overrides auto-detect)

```bash
go run . -repo neo4j/mcp \
  -assets neo4j-mcp_1.4.2_Linux_x86_64.tar.gz,neo4j-mcp_1.4.2_Darwin_arm64.tar.gz
```

### Use a custom package name and entry point

```bash
go run . -repo acme/mytool \
  -binary-name mytool \
  -package-name acme-mytool \
  -entry-point mytool \
  -summary "Acme Corp's CLI tool, packaged as a Python wheel"
```

### Build and upload to PyPI

```bash
PYPI_TOKEN=pypi-xxxxxxxxxxxx go run . -repo neo4j/mcp -binary-name neo4j-mcp -upload
```

### Test against TestPyPI first (recommended)

```bash
PYPI_TOKEN=pypi-xxxxxxxxxxxx go run . -repo neo4j/mcp -binary-name neo4j-mcp \
  -upload \
  -pypi-url https://test.pypi.org/legacy/
```

Then verify the install from TestPyPI:

```bash
pip install --index-url https://test.pypi.org/simple/ neo4j-mcp
uv tool install --index-url https://test.pypi.org/simple/ neo4j-mcp
neo4j-mcp -v
```

### Enable debug logging

All log output goes to stderr; stdout is kept clean for scripted use:

```bash
go run . -repo neo4j/mcp -binary-name neo4j-mcp -debug 2>build.log
```

### Compile for repeated use

```bash
go build -o buildwheels
./buildwheels -repo neo4j/mcp -binary-name neo4j-mcp -upload
```

---

## Running tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run a specific test file / function
go test -v -run TestBuildWheel
go test -v -run TestFetchRelease

# Run tests with the race detector
go test -race ./...
```

Tests use only the Go standard library (`testing`, `net/http/httptest`, `os`) and run fully offline — no GitHub API or PyPI calls are made.

---

## Project structure

```
.
├── main.go          # CLI entry point and pipeline orchestration (run)
├── config.go        # Config struct and defaultPyPIURL constant
├── log.go           # Structured logging setup (log/slog → stderr)
├── github.go        # GitHub Releases API client
├── download.go      # HTTP download with optional on-disk caching
├── archive.go       # Binary extraction from .tar.gz and .zip archives
├── platform.go      # Platform map, asset resolution, GoReleaser name conventions
├── wheel.go         # Python wheel construction (zip layout, shim, RECORD)
├── files.go         # License and description file resolution
├── pypi.go          # PyPI legacy upload endpoint client
├── go.mod           # Go module definition
├── DESCRIPTION.md   # Default PyPI long description (Markdown)
├── LICENSE          # Licence for this tool itself
└── dist/            # Default wheel output directory
```

Test files (`*_test.go`) sit alongside the source files they test.

---

## GitHub Actions

The included workflow (`gha_workflow.txt`, rename to `.github/workflows/release.yml`) automates the full pipeline. It can be triggered manually, on a schedule, or via a `repository_dispatch` event from the upstream release repository.

### Jobs

**check-version** — runs first for all trigger types. Resolves which tag to use (from workflow input, webhook payload, or the GitHub API), then checks PyPI to see if that version is already published. If it is, all downstream jobs are skipped, so the daily schedule doesn't re-publish the same version repeatedly.

**build** — runs `go run .` and uploads the wheels as a GitHub Actions artifact. Uses `actions/cache` keyed on the binary version so repeated runs for the same release don't re-download archives. Separating build from publish means you can inspect the wheels before they reach PyPI.

**publish** — downloads the artifact and pushes to PyPI. The `if:` condition controls when publishing fires:
- `workflow_dispatch` — only publishes when you explicitly set `upload: true`
- `repository_dispatch` / `schedule` — always publishes (triggered by real release events)

Configured for OIDC trusted publishing by default (no stored secret needed). Switching to an API token requires uncommenting one line.

**notify** — opens a GitHub issue automatically if any job fails.

### Setup checklist

1. **PyPI trusted publishing** — go to your PyPI project → Publishing → add a GitHub Actions publisher with your owner, repo name, and workflow name `release.yml`.
2. **Webhook from the upstream repo** (optional but recommended over polling) — add this step to the upstream release workflow:

```yaml
- name: Trigger wheels build
  run: |
    curl -X POST \
      -H "Authorization: Bearer ${{ secrets.WHEELS_REPO_PAT }}" \
      -H "Accept: application/vnd.github+json" \
      https://api.github.com/repos/your-org/buildwheels/dispatches \
      -d '{"event_type":"new-mcp-release","client_payload":{"tag":"${{ github.ref_name }}"}}'
```

3. **Environment protection** (optional) — create a `pypi` environment in Settings → Environments and add required reviewers for a manual approval gate before publishing.

### Trigger manually

From the **Actions** tab, select **Build and Publish Wheels**, click **Run workflow**, and optionally enter a specific release tag. Leave the tag blank to use the latest release.

---

## PyPI setup

### Option A — Trusted publishing (OIDC, no stored secrets)

1. Log in to [pypi.org](https://pypi.org) and create your project
2. Under **Publishing**, add a **GitHub Actions** trusted publisher:
   - Owner: `your-org`
   - Repository: `buildwheels`
   - Workflow: `release.yml`
3. Ensure the publish job has `id-token: write` permission (already included in the workflow)
4. No `PYPI_TOKEN` secret is needed — OIDC handles auth automatically

### Option B — API token

1. Log in to [pypi.org](https://pypi.org) → **Account settings → API tokens**
2. Create a token scoped to your project
3. Add it as `PYPI_TOKEN` in your repository secrets

---

## Troubleshooting

### Asset not found

The tool logs a warning with the expected asset filename and skips that platform. Check the upstream releases page to confirm the actual archive filenames. If the naming convention differs from GoReleaser defaults, use `-assets` to supply the exact filenames explicitly.

### GitHub rate limit (403)

Set `GITHUB_TOKEN` with a personal access token to raise the limit from 60 to 5,000 requests per hour.

### PyPI 400 — File already exists

This is non-fatal. The tool logs a warning and continues. PyPI does not allow overwriting a published release; bump the Python package version with `-py-version` instead.

### Wheel installs but binary does not run

Verify the executable bit is set correctly — the shim relies on `os.execv` (Unix) or `subprocess` (Windows) to hand off to the bundled binary. Re-running the build and reinstalling usually resolves this. Run with `-debug` to see exactly which binary path the shim is constructing.

### Platform tag not accepted by installer

`pip`, `pipx`, `uv` etc. only accept wheels whose tags match the current platform. List the tags your interpreter accepts:

```python
import platform, subprocess, sys

print(platform.platform())

result = subprocess.run(
    [sys.executable, "-m", "pip", "debug", "--verbose"],
    capture_output=True, text=True
)
print(result.stdout)
```

Find a wheel filename in the output that matches the `.whl` filename you are trying to install.

---

## Notes on wheel file construction

### Zip format compatibility

Three zip-related edge cases are handled explicitly:

| Problem | Symptom | Fix applied |
|---|---|---|
| `zip.Deflate` data descriptors | PyPI/twine rejects with `400 Invalid distribution` | `CreateRaw` with pre-computed CRC/sizes and `Flags=0` |
| Empty `RECORD` | `uv` rejects wheel as malformed | Two-pass build: collect all entries first, then compute SHA-256 hashes |
| Missing 32-bit size fields | `uv` skips entries with zip64 fields; `WHEEL not found` | Both `CompressedSize` and `CompressedSize64` are populated |

### SPDX licence expression

The `-license-expr` value is embedded directly in wheel `METADATA` as `License-Expression`. It must be a valid [SPDX identifier](https://spdx.org/licenses/) (e.g. `MIT`, `Apache-2.0`, `GPL-3.0-or-later`). PyPI will reject uploads with an unrecognised value.

### Wheel compatibility tag

Wheels are tagged `py3-none-<platform>` — compatible with any CPython 3.x interpreter on the target platform, with no ABI dependency. This is correct for wheels that bundle a self-contained native binary.

### Strict installers

`uv` / `uvx` are significantly stricter than `pip` / `pipx` about wheel spec compliance. Use `uv tool install <path-to-.whl>` as a quick validation step before publishing.
