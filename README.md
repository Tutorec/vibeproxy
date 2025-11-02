# VibeProxy

<p align="center">
  <img src="icon.png" width="128" height="128" alt="VibeProxy Icon">
</p>

<p align="center">
<a href="https://automaze.io" rel="nofollow"><img alt="Automaze" src="https://img.shields.io/badge/By-automaze.io-4b3baf" style="max-width: 100%;"></a>
<a href="https://github.com/automazeio/vibeproxy/blob/main/LICENSE"><img alt="MIT License" src="https://img.shields.io/badge/License-MIT-28a745" style="max-width: 100%;"></a>
<a href="http://x.com/intent/follow?screen_name=aroussi" rel="nofollow"><img alt="Follow on 𝕏" src="https://img.shields.io/badge/Follow-%F0%9D%95%8F/@aroussi-1c9bf0" style="max-width: 100%;"></a>
<a href="https://github.com/automazeio/vibeproxy"><img alt="Star this repo" src="https://img.shields.io/github/stars/automazeio/vibeproxy.svg?style=social&amp;label=Star%20this%20repo&amp;maxAge=60" style="max-width: 100%;"></a></p>
</p>

**Stop paying twice for AI.** VibeProxy lets you use your existing Claude Code, ChatGPT, **Gemini**, and **Qwen** subscriptions with powerful AI coding tools like **[Factory Droids](https://app.factory.ai/r/FM8BJHFQ)** – no separate API keys required.

**Cross-platform Go implementation** works on **Linux**, **macOS**, and **Windows** with a browser-based UI. Built on [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI), it handles OAuth authentication, token management, and API routing automatically. One click to authenticate, zero friction to code.

> [!IMPORTANT]
> **NEW: Gemini and Qwen Support! 🎉** VibeProxy now supports Google's Gemini AI and Qwen AI with full OAuth authentication. Connect your accounts and use Gemini and Qwen with your favorite AI coding tools!

> [!IMPORTANT]
> **NEW: Extended Thinking Support! 🧠** VibeProxy now supports Claude's extended thinking feature with dynamic budgets (4K, 10K, 32K tokens). Use model names like `claude-sonnet-4-5-20250929-thinking-10000` to enable extended thinking. See the [Factory Setup Guide](FACTORY_SETUP.md#step-3-configure-factory-cli) for details.

<p align="center">
<br>
  <a href="https://www.loom.com/share/5cf54acfc55049afba725ab443dd3777"><img src="vibeproxy-factory-video.webp" width="600" height="380" alt="VibeProxy Screenshot" border="0"></a>
</p>

> [!TIP]
> Check out our [Factory Setup Guide](FACTORY_SETUP.md) for step-by-step instructions on how to use VibeProxy with Factory Droids.


## Features

- 🌐 **Cross-Platform** - Works on Linux, macOS, and Windows
- 🚀 **Zero-Config Setup** - Auto-downloads dependencies and creates config
- 🔐 **OAuth Integration** - Authenticate with Claude Code, Codex, Gemini, and Qwen
- 📊 **Real-Time Status** - Live connection status and automatic credential detection
- 🔄 **Auto-Updates** - Monitors auth files and updates UI in real-time
- 🎨 **Browser UI** - Modern web interface accessible at localhost:8319
- 💾 **Self-Contained** - Single binary with everything embedded
- 🧠 **Extended Thinking** - Claude's extended thinking with dynamic token budgets (4K/10K/32K)


## Installation

### Prerequisites

- **Go 1.21+** (for building from source)
- **curl** and **tar** (for downloading dependencies)
- **Browser** (any modern browser for the UI)

### Quick Start (All Platforms)

```bash
# Clone the repository
git clone https://github.com/automazeio/vibeproxy.git
cd vibeproxy

# Build (auto-detects your OS and architecture)
make build

# Run
./vibeproxy
```

**What happens:**
1. Downloads the appropriate `cli-proxy-api` binary for your platform
2. Creates `config.yaml` from default template
3. Builds the Go binary
4. Starts all services (ThinkingProxy, CLIProxyAPI, Web UI)
5. Auto-opens browser to `http://localhost:8319/static/`

### Platform-Specific Notes

#### Linux
```bash
# System-wide installation
make install

# Create .deb package (Debian/Ubuntu)
make package
```

See [**LINUX.md**](LINUX.md) for autostart setup and troubleshooting.

#### macOS
```bash
# Build and run
make build
./vibeproxy

# System-wide installation
make install
```

Works on both Intel and Apple Silicon (M1/M2/M3/M4).

#### Windows
Support planned. CLIProxyAPI has Windows binaries available, but browser auto-launch needs implementation.

## Usage

### First Launch

1. Run `./vibeproxy` (or `vibeproxy` if installed system-wide)
2. Browser opens automatically to `http://localhost:8319/static/`
3. Click "Connect" for Claude Code, Codex, Gemini, or Qwen
4. Complete OAuth in the browser
5. VibeProxy detects your credentials automatically

### Authentication

When you click "Connect":
1. Browser opens with the OAuth page
2. Complete authentication (login/authorize)
3. Close the browser when done
4. VibeProxy automatically detects credentials
5. UI shows "Connected" status

### Server Management

- **Status**: Green = running and healthy (port 8318 responding)
- **Background Mode**: Close browser, proxy keeps running
- **Stop Server**: Press Ctrl+C in terminal
- **Launch at Login**: Toggle in web UI (Linux: creates XDG autostart entry)

### Using with Your IDE

Configure your IDE/tools to use:
```
API Endpoint: http://localhost:8317
Model: claude-sonnet-4-5-20250929-thinking-10000
```

The `-thinking-BUDGET` suffix enables extended thinking (2K/10K/32K tokens).

## Development

### Project Structure

```
vibeproxy/
├── cmd/vibeproxy/           # Main entry point
│   └── main.go              # Orchestrates all services
├── internal/
│   ├── auth/                # Auth file parsing & watching
│   │   ├── status.go        # JSON credential parser
│   │   └── watcher.go       # fsnotify file watcher
│   ├── process/             # CLIProxyAPI process management
│   │   └── manager.go       # Start/stop/health check
│   ├── proxy/               # ThinkingProxy HTTP interceptor
│   │   └── thinking.go      # Model name transformation
│   └── server/              # Web UI server
│       ├── ui.go            # HTTP endpoints (status/connect/disconnect)
│       └── static/          # Browser UI assets
│           ├── index.html
│           ├── style.css
│           └── app.js
├── config.default.yaml      # Default config template (port 8318)
├── go.mod                   # Go module definition
├── go.sum                   # Dependency checksums
└── Makefile                 # Cross-platform build automation
```

### Key Components

- **main.go**: Starts ThinkingProxy (8317), CLIProxyAPI (8318), Web UI (8319), and file watcher
- **process.Manager**: Manages cli-proxy-api lifecycle with health checks
- **proxy.ThinkingProxy**: HTTP reverse proxy with model name transformation
- **auth.Watcher**: Monitors `~/.cli-proxy-api/` for credential changes
- **server.UIServer**: Serves web UI and API endpoints

## Credits

VibeProxy is built on top of [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI), an excellent unified proxy server for AI services.

Special thanks to the CLIProxyAPI project for providing the core functionality that makes VibeProxy possible.

## License

MIT License - see LICENSE file for details

## Support

- **Report Issues**: [GitHub Issues](https://github.com/automazeio/vibeproxy/issues)
- **Website**: [automaze.io](https://automaze.io)

---

© 2025 [Automaze, Ltd.](https://automaze.io) All rights reserved.
