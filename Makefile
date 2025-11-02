.PHONY: build install clean run help

help: ## Show this help message
	@echo "VibeProxy - Cross-Platform OAuth Proxy (Go)"
	@echo ""
	@echo "Build targets:"
	@echo "  make build          - Build for current platform"
	@echo "  make run            - Build and run"
	@echo "  make install        - Install to system"
	@echo "  make package        - Create distribution package"
	@echo ""
	@echo "Common targets:"
	@echo "  make clean          - Clean all build artifacts"
	@echo "  make help           - Show this help"

# Default target - build for current platform
build: ## Build for current platform
	@echo "🔨 Building VibeProxy..."
	@echo "📥 Checking for cli-proxy-api binary..."
	@if [ ! -f cli-proxy-api ]; then \
		OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
		ARCH=$$(uname -m); \
		if [ "$$ARCH" = "x86_64" ]; then \
			ARCH_NAME="amd64"; \
		elif [ "$$ARCH" = "aarch64" ] || [ "$$ARCH" = "arm64" ]; then \
			ARCH_NAME="arm64"; \
		else \
			echo "❌ Unsupported architecture: $$ARCH"; \
			exit 1; \
		fi; \
		if [ "$$OS" = "darwin" ]; then \
			OS_NAME="darwin"; \
		elif [ "$$OS" = "linux" ]; then \
			OS_NAME="linux"; \
		else \
			echo "❌ Unsupported OS: $$OS"; \
			exit 1; \
		fi; \
		echo "📥 Downloading CLIProxyAPI for $$OS_NAME $$ARCH_NAME..."; \
		curl -L -o cli-proxy-api.tar.gz "https://github.com/router-for-me/CLIProxyAPI/releases/download/v6.3.4/CLIProxyAPI_6.3.4_$${OS_NAME}_$${ARCH_NAME}.tar.gz"; \
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

run: build ## Build and run
	@echo "🚀 Running VibeProxy..."
	@./vibeproxy

install: build ## Install to system
	@echo "📲 Installing to /usr/local/bin..."
	@sudo cp vibeproxy /usr/local/bin/
	@sudo cp cli-proxy-api /usr/local/bin/
	@sudo chmod +x /usr/local/bin/vibeproxy
	@sudo chmod +x /usr/local/bin/cli-proxy-api
	@echo "✅ Installed to /usr/local/bin/vibeproxy"

package: build ## Create .deb package (Debian/Ubuntu)
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
	@rm -f vibeproxy
	@rm -f cli-proxy-api
	@rm -f vibeproxy_*.deb
	@rm -rf package
	@echo "✅ Clean complete"
