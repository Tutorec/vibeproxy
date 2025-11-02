.PHONY: build app install clean run help linux-build linux-install linux-run

help: ## Show this help message
	@echo "VibeProxy - Multi-Platform OAuth Proxy"
	@echo ""
	@echo "macOS (Swift) targets:"
	@echo "  make build       - Build macOS Swift executable"
	@echo "  make app         - Create macOS .app bundle"
	@echo "  make install     - Install to /Applications"
	@echo "  make run         - Build and run macOS app"
	@echo ""
	@echo "Linux (Go) targets:"
	@echo "  make linux-build - Build Linux Go executable"
	@echo "  make linux-run   - Build and run Linux app"
	@echo "  make linux-install - Install to /usr/local/bin"
	@echo ""
	@echo "Common targets:"
	@echo "  make clean       - Clean all build artifacts"
	@echo "  make help        - Show this help"

# macOS targets (Swift)
build: ## Build the Swift executable (debug)
	@echo "🔨 Building Swift executable..."
	@cd src && swift build
	@echo "✅ Build complete: src/.build/debug/CLIProxyMenuBar"

release: ## Build the Swift executable (release)
	@echo "🔨 Building Swift executable (release)..."
	@./build.sh
	@echo "✅ Build complete: src/.build/release/CLIProxyMenuBar"

app: ## Create the .app bundle
	@echo "📦 Creating .app bundle..."
	@./create-app-bundle.sh
	@echo "✅ App bundle created: VibeProxy.app"

install: app ## Build and install to /Applications
	@echo "📲 Installing to /Applications..."
	@rm -rf "/Applications/VibeProxy.app"
	@cp -r "VibeProxy.app" /Applications/
	@echo "✅ Installed to /Applications/VibeProxy.app"

run: app ## Build and run the app
	@echo "🚀 Launching app..."
	@open "VibeProxy.app"

# Linux targets (Go)
linux-build: ## Build Linux Go executable
	@echo "🔨 Building Go executable for Linux..."
	@echo "📥 Checking for cli-proxy-api binary..."
	@if [ ! -f cli-proxy-api ]; then \
		ARCH=$$(uname -m); \
		if [ "$$ARCH" = "x86_64" ]; then \
			ARCH_NAME="amd64"; \
		elif [ "$$ARCH" = "aarch64" ] || [ "$$ARCH" = "arm64" ]; then \
			ARCH_NAME="arm64"; \
		else \
			echo "❌ Unsupported architecture: $$ARCH"; \
			exit 1; \
		fi; \
		echo "📥 Downloading CLIProxyAPI for Linux $$ARCH_NAME..."; \
		curl -L -o cli-proxy-api.tar.gz "https://github.com/router-for-me/CLIProxyAPI/releases/download/v6.3.4/CLIProxyAPI_6.3.4_linux_$$ARCH_NAME.tar.gz"; \
		tar -xzf cli-proxy-api.tar.gz cli-proxy-api; \
		chmod +x cli-proxy-api; \
		rm cli-proxy-api.tar.gz; \
		echo "✅ CLIProxyAPI downloaded and extracted"; \
	else \
		echo "✅ cli-proxy-api already exists, skipping download"; \
	fi
	@echo "📝 Setting up config.yaml..."
	@if [ ! -f config.yaml ]; then \
		cp config.default.yaml config.yaml; \
		echo "✅ config.yaml created from default template"; \
	else \
		echo "✅ config.yaml already exists"; \
	fi
	@go build -o vibeproxy ./cmd/vibeproxy
	@echo "✅ Build complete: ./vibeproxy"

linux-run: linux-build ## Build and run Linux app
	@echo "🚀 Running VibeProxy..."
	@./vibeproxy

linux-install: linux-build ## Install to /usr/local/bin
	@echo "📲 Installing to /usr/local/bin..."
	@sudo cp vibeproxy /usr/local/bin/
	@sudo cp cli-proxy-api /usr/local/bin/
	@sudo chmod +x /usr/local/bin/vibeproxy
	@sudo chmod +x /usr/local/bin/cli-proxy-api
	@echo "✅ Installed to /usr/local/bin/vibeproxy"

linux-package: linux-build ## Create .deb package
	@echo "📦 Creating .deb package..."
	@mkdir -p package/DEBIAN
	@mkdir -p package/usr/local/bin
	@mkdir -p package/usr/share/applications
	@cp vibeproxy package/usr/local/bin/
	@cp cli-proxy-api package/usr/local/bin/
	@chmod +x package/usr/local/bin/cli-proxy-api
	@echo "Package: vibeproxy\nVersion: 1.0.5\nSection: utils\nPriority: optional\nArchitecture: amd64\nMaintainer: Automaze Ltd <hello@automaze.io>\nDescription: OAuth Authentication Proxy for AI Services\n Simple OAuth proxy for Claude, Codex, Gemini, and Qwen." > package/DEBIAN/control
	@dpkg-deb --build package vibeproxy_1.0.5_amd64.deb
	@rm -rf package
	@echo "✅ Package created: vibeproxy_1.0.5_amd64.deb"

clean: ## Clean build artifacts
	@echo "🧹 Cleaning..."
	@rm -rf src/.build
	@rm -rf "VibeProxy.app"
	@rm -rf src/Sources/Resources/cli-proxy-api
	@rm -rf src/Sources/Resources/config.yaml
	@rm -rf src/Sources/Resources/static
	@rm -f vibeproxy
	@rm -f cli-proxy-api
	@rm -f vibeproxy_*.deb
	@rm -rf package
	@echo "✅ Clean complete"

test: ## Run a quick test build
	@echo "🧪 Testing build..."
	@cd src && swift build
	@echo "✅ Test build successful"

info: ## Show project information
	@echo "Project: VibeProxy - macOS Menu Bar App"
	@echo "Language: Swift 5.9+"
	@echo "Platform: macOS 13.0+"
	@echo ""
	@echo "Files:"
	@find src/Sources -name "*.swift" -exec wc -l {} + | tail -1 | awk '{print "  Swift code: " $$1 " lines"}'
	@echo "  Documentation: 4 files"
	@echo ""
	@echo "Structure:"
	@tree -L 3 -I ".build" || echo "  (install 'tree' for better output)"

open: ## Open app bundle to inspect contents
	@if [ -d "VibeProxy.app" ]; then \
		open "VibeProxy.app"; \
	else \
		echo "❌ App bundle not found. Run 'make app' first."; \
	fi

edit-config: ## Edit the bundled config.yaml
	@if [ -d "VibeProxy.app" ]; then \
		open -e "VibeProxy.app/Contents/Resources/config.yaml"; \
	else \
		echo "❌ App bundle not found. Run 'make app' first."; \
	fi

# Shortcuts
all: app ## Same as 'app'
