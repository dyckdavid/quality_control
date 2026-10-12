# Quality Control Desktop

Native desktop client for the quality-control API. Built with **Rust + egui/eframe** (GPU immediate-mode UI).

## Why egui/eframe (not Slint)

| Criterion | egui/eframe | Slint |
| --- | --- | --- |
| Runs on this Linux VM | Yes — OpenGL via glow, easy under Xvfb | Yes, but needs Slint tooling/DSL |
| Language | Pure Rust UI code | `.slint` declarative DSL + Rust |
| Cross-platform | macOS + Linux (and Windows) without webviews | Same |
| Fit for data tables | Immediate-mode grids/search/sort are straightforward | Excellent for designed UIs; more setup |

We chose **egui/eframe** so the entire client stays in Rust, builds with a normal `cargo` toolchain on Linux and macOS, and supports automated screenshots under Xvfb without embedding a webview (no Electron/Tauri).

## Features

- Sidebar: Dashboard, Inspections, Nonconformances, Corrective Actions, Settings
- Sortable / searchable tables with loading, empty, and error states
- Detail views and create/edit forms with client-side validation
- Keyboard shortcuts (see in-app Settings)
- CSV export for list pages
- Light / dark / system theme
- Persisted server URL (platform config dir)
- HTTP on background threads via `poll-promise` (UI never blocks on the network)

## Requirements

- Rust 1.78+ (edition 2021)
- Linux: OpenGL / X11 (or Wayland) libraries; for headless screenshots: `xvfb`
- A running quality-control API (see repo root `README.md`)

### Linux packages (Debian/Ubuntu)

```bash
sudo apt install build-essential pkg-config libxcb-render0-dev \
  libxcb-shape0-dev libxcb-xfixes0-dev libxkbcommon-dev libxkbcommon-x11-0 \
  libssl-dev libgtk-3-dev xvfb
```

### macOS

Install a recent Xcode Command Line Tools / Rust via rustup. No extra GUI toolkit install is required for egui’s glow backend.

## Build

```bash
cd desktop
cargo build --release
```

Binary: `target/release/qc-desktop`

## Run

Start the API first (from the repo root), for example:

```bash
# Postgres + API via docker compose, or:
export QC_DATABASE_URL='postgres://qc:qc-local-password@127.0.0.1:5432/quality_control?sslmode=disable'
export QC_JWT_SECRET='local-dev-jwt-secret-at-least-32-chars-long!!'
export QC_ADMIN_PASSWORD='change-me-now'
go run ./cmd/quality-control
```

Then:

```bash
cd desktop
cargo run --release -- --server http://127.0.0.1:8080
```

Sign in with the admin credentials configured on the server (`QC_ADMIN_USERNAME` / `QC_ADMIN_PASSWORD`).

### CLI flags

| Flag | Purpose |
| --- | --- |
| `--server URL` | Override API base URL |
| `--screenshot-dir DIR` | Capture main screens then exit |
| `--user` / `--password` | Auto-login for screenshots (`QC_DESKTOP_USER` / `QC_DESKTOP_PASSWORD` env also work) |

## Tests

Unit tests (no server required):

```bash
cd desktop
cargo test --lib
```

Integration tests (API must be reachable):

```bash
export QC_API_BASE_URL=http://127.0.0.1:8080
export QC_API_USERNAME=admin
export QC_API_PASSWORD=change-me-now
cargo test --tests -- --nocapture
```

## Screenshots under Xvfb (Linux)

```bash
cd desktop
mkdir -p screenshots
xvfb-run -a cargo run --release -- \
  --server http://127.0.0.1:8080 \
  --user admin \
  --password change-me-now \
  --screenshot-dir ./screenshots
```

PNGs land in `screenshots/` (`01_login.png` … `06_settings.png`).

## Keyboard shortcuts

| Shortcut | Action |
| --- | --- |
| Ctrl/Cmd+1…5 | Dashboard / Inspections / NCRs / CAPAs / Settings |
| Ctrl/Cmd+R | Refresh current view |
| Ctrl/Cmd+N | New record (list pages) |
| Ctrl/Cmd+, | Settings |
| Esc | Close detail / form |
| Enter | Submit login |

## Settings location

Persisted JSON via the `directories` crate, typically:

- Linux: `~/.config/qc-desktop/settings.json`
- macOS: `~/Library/Application Support/com.QualityControl.qc-desktop/settings.json`

## CI

GitHub Actions workflow `.github/workflows/desktop.yml` builds on `ubuntu-latest` and `macos-latest`, runs unit tests, and (on Ubuntu) starts Postgres + the Go API for integration tests.
