# VibeProxy for Linux

**Browser-based OAuth authentication proxy for AI services on Debian-based Linux distributions.**

## What is VibeProxy?

VibeProxy is a local proxy server that unifies OAuth authentication for multiple AI services (Claude Code, Codex, Gemini, and Qwen) so you don't need duplicate subscriptions. It runs in the background and provides:

- **ThinkingProxy** on port 8317 - Client-facing proxy with extended thinking support
- **CLIProxyAPI** on port 8318 - Backend API server
- **Web UI** on port 8319 - Browser-based configuration interface

## Architecture

```
Your IDE/Client (port 8317)
    ↓
ThinkingProxy (Go) - HTTP interceptor with thinking parameter transformation
    ↓
CLIProxyAPI (Go binary) - OAuth management and API routing
    ↓
AI Services (Claude, Codex, Gemini, Qwen)
```

## Installation

### Prerequisites

- Debian-based Linux (Ubuntu, Debian, Linux Mint, etc.)
- Go 1.21+ (for building from source)
- `xdg-utils` package (for browser auto-launch)
- `curl` and `tar` (for downloading CLIProxyAPI binary during build)

### Option 1: Build from Source

```bash
# Clone the repository
git clone https://github.com/automazeio/vibeproxy.git
cd vibeproxy

# Build the Linux binary
make linux-build

# Run directly
./vibeproxy
```

**What happens during build:**
- The Makefile automatically detects your CPU architecture (amd64/arm64)
- Downloads the appropriate `cli-proxy-api` binary from [CLIProxyAPI releases](https://github.com/router-for-me/CLIProxyAPI/releases)
- Creates `config.yaml` from the default template if it doesn't exist
- Builds the Go vibeproxy wrapper
- Both binaries and config must be in the same directory

**Auto-Configuration:**
If `config.yaml` doesn't exist when you run vibeproxy, it will automatically:
1. Look for `config.default.yaml` and copy it
2. If no template exists, create a minimal working config
3. Always use port 8318 for CLIProxyAPI backend (correct port configuration)

### Option 2: Install to System

```bash
# Build and install to /usr/local/bin
make linux-install

# Run from anywhere
vibeproxy
```

### Option 3: Create .deb Package

```bash
# Create Debian package
make linux-package

# Install the package
sudo dpkg -i vibeproxy_1.0.5_amd64.deb
```

## Usage

### Starting VibeProxy

```bash
# Run the binary (from source directory)
./vibeproxy

# OR if installed system-wide
vibeproxy
```

**What happens:**
1. All three services start (ThinkingProxy, CLIProxyAPI, Web UI)
2. Your default browser opens to `http://localhost:8319/static/`
3. The binary continues running in the background

### Configuring Authentication

1. **Open the Web UI**: `http://localhost:8319/static/`
2. **Click "Connect"** on any service (Claude, Codex, Gemini, Qwen)
3. **Complete OAuth** in the browser window that opens
4. **Close the browser** - the proxy keeps running

### Using with Your IDE

Configure your IDE/tools to use `http://localhost:8317` as the API endpoint.

**Example for Claude Code:**
```json
{
  "api_endpoint": "http://localhost:8317",
  "model": "claude-sonnet-4-5-20250929-thinking-10000"
}
```

The `-thinking-10000` suffix enables extended thinking with a 10,000 token budget.

### Checking Status

Just open the web UI again: `http://localhost:8319/static/`

You'll see:
- Server running status (green = running)
- Authentication status for each service
- Connect/Disconnect/Reconnect buttons

### Stopping VibeProxy

Press `Ctrl+C` in the terminal where it's running.

## Autostart on Boot

The web UI has a "Launch at login" toggle that creates an XDG autostart entry.

**Manual setup:**

```bash
# Create autostart directory
mkdir -p ~/.config/autostart

# Create desktop entry
cat > ~/.config/autostart/vibeproxy.desktop << 'EOF'
[Desktop Entry]
Type=Application
Name=VibeProxy
Exec=/usr/local/bin/vibeproxy
Hidden=false
NoDisplay=false
X-GNOME-Autostart-enabled=true
EOF
```

## Disconnecting Services

1. Open web UI: `http://localhost:8319/static/`
2. Click "Disconnect" next to any service
3. Confirm the action

This deletes the auth file from `~/.cli-proxy-api/`.

## File Locations

- **Auth files**: `~/.cli-proxy-api/*.json`
- **Binary**: `./vibeproxy` (or `/usr/local/bin/vibeproxy` if installed)
- **Config**: Embedded in binary (no external config needed)

## Ports

- **8317** - ThinkingProxy (use this in your IDE) - Client-facing proxy with extended thinking
- **8318** - CLIProxyAPI (internal, do not use directly) - Backend API server
- **8319** - Web UI (browser interface) - Configuration and status

**IMPORTANT**: Your `config.yaml` must have `port: 8318` for CLIProxyAPI. The auto-generated config sets this correctly. If you manually edit the config, do NOT change this port value.

## Extended Thinking Support

Add `-thinking-BUDGET` to any Claude model name:

| Model Name | Budget |
|------------|--------|
| `claude-sonnet-4-5-20250929-thinking-2000` | 2,000 tokens |
| `claude-sonnet-4-5-20250929-thinking-10000` | 10,000 tokens |
| `claude-sonnet-4-5-20250929-thinking-32000` | 32,000 tokens (max) |

The proxy automatically:
1. Strips the suffix from the model name
2. Adds the `thinking` parameter to the request
3. Ensures `max_tokens` > budget

## Troubleshooting

### Browser doesn't open automatically

Manually open: `http://localhost:8319/static/`

### Port already in use

Check what's using the ports:
```bash
sudo lsof -i :8317
sudo lsof -i :8318
sudo lsof -i :8319
```

Kill the process or change VibeProxy ports in the source code.

### OAuth fails

1. Check that the browser opened successfully
2. Ensure you completed the OAuth flow
3. Try disconnecting and reconnecting
4. Check logs in the terminal where VibeProxy is running

### Auth file issues

Check auth directory:
```bash
ls -la ~/.cli-proxy-api/
```

Files should be named like `claude.json`, `codex.json`, etc.

### Binary not found

If you see "cli-proxy-api binary not found":

1. **Check if cli-proxy-api exists**: `ls -l cli-proxy-api`
2. **Re-run the build**: `make clean && make linux-build`
3. **Manual download** (if build fails):
   ```bash
   # For x86-64 systems
   curl -L -o cli-proxy-api.tar.gz https://github.com/router-for-me/CLIProxyAPI/releases/download/v6.3.4/CLIProxyAPI_6.3.4_linux_amd64.tar.gz
   tar -xzf cli-proxy-api.tar.gz cli-proxy-api
   chmod +x cli-proxy-api
   rm cli-proxy-api.tar.gz

   # For ARM64 systems
   curl -L -o cli-proxy-api.tar.gz https://github.com/router-for-me/CLIProxyAPI/releases/download/v6.3.4/CLIProxyAPI_6.3.4_linux_arm64.tar.gz
   tar -xzf cli-proxy-api.tar.gz cli-proxy-api
   chmod +x cli-proxy-api
   rm cli-proxy-api.tar.gz
   ```

The vibeproxy binary looks for `cli-proxy-api` in the same directory as the executable.

For system installations, both binaries are installed to `/usr/local/bin/`.

### Config file issues

**VibeProxy auto-creates config.yaml if missing**. If you have config issues:

1. **Delete and regenerate**: `rm config.yaml && ./vibeproxy`
2. **Check port configuration**: `grep "^port:" config.yaml` should show `port: 8318`
3. **Wrong port value**: If you see `port: 8317`, change it to `port: 8318`

The backend CLIProxyAPI must listen on port 8318. ThinkingProxy forwards from 8317 to 8318.

## Development

### Project Structure

```
vibeproxy/
├── cmd/vibeproxy/          # Main entry point
│   └── main.go
├── internal/
│   ├── auth/               # Auth file parsing & watching
│   │   ├── status.go
│   │   └── watcher.go
│   ├── process/            # CLIProxyAPI process management
│   │   └── manager.go
│   ├── proxy/              # ThinkingProxy HTTP interceptor
│   │   └── thinking.go
│   └── server/             # Web UI server
│       ├── ui.go
│       └── static/         # HTML/CSS/JS files
│           ├── index.html
│           ├── style.css
│           └── app.js
├── go.mod
├── go.sum
├── Makefile
└── LINUX.md                # This file
```

### Building

```bash
# Debug build
go build -o vibeproxy ./cmd/vibeproxy

# Release build with optimizations
go build -ldflags="-s -w" -o vibeproxy ./cmd/vibeproxy
```

### Running Tests

```bash
# Type check
go build ./...

# Run with verbose logging
go run ./cmd/vibeproxy
```

## Cross-Platform Implementation

VibeProxy is now a **pure Go implementation** that works across platforms:

| Feature | Implementation |
|---------|----------------|
| UI | Browser-based (HTML/CSS/JS) |
| System Tray | None (web UI instead) |
| Autostart | XDG autostart (Linux), launchd (macOS planned) |
| Distribution | Single binary (~9 MB) + cli-proxy-api (~32 MB) |
| Dependencies | fsnotify (Go library) |
| Platforms | Linux ✅, macOS ✅, Windows (planned) |

The Go implementation provides identical functionality across all platforms with zero platform-specific code.

## License

MIT License - see LICENSE file

## Credits

- Built on top of [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)
- Developed by [Automaze, Ltd.](https://automaze.io)

## Support

- Report issues: https://github.com/automazeio/vibeproxy/issues
- Documentation: https://github.com/automazeio/vibeproxy
