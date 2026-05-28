#!/bin/bash
set -e

# ============================================================
# RepoMind 一键安装脚本
# Usage: ./install.sh [release-url]
#
# 默认从 OSS 下载最新版本，也支持自定义 URL。
#
# 脚本会自动:
#   1. 检测 Linux / macOS + amd64 / arm64
#   2. 从 <release-url>/repomind-<os>-<arch> 下载可执行文件
#   3. 安装到 /usr/local/bin 或 ~/.local/bin
#   4. 自动配置 PATH，立即可用
# ============================================================

RELEASE_URL="${1:-https://nemo-res.oss-ap-southeast-1.aliyuncs.com/codeai}"

# Remove trailing slash
RELEASE_URL="${RELEASE_URL%/}"

# --- Detect OS ---
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    linux)  GOOS="linux" ;;
    darwin) GOOS="darwin" ;;
    *)      echo "Unsupported OS: $OS (only Linux and macOS are supported)"; exit 1 ;;
esac

# --- Detect Arch ---
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  GOARCH="amd64" ;;
    aarch64) GOARCH="arm64" ;;
    arm64)   GOARCH="arm64" ;;
    *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

BINARY_NAME="repomind-${GOOS}-${GOARCH}"
DOWNLOAD_URL="${RELEASE_URL}/${BINARY_NAME}"

echo "Detected: ${GOOS}/${GOARCH}"
echo "Downloading: ${DOWNLOAD_URL}"

# --- Choose install dir ---
if [ -w /usr/local/bin ]; then
    INSTALL_DIR="/usr/local/bin"
else
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
fi

# --- Download ---
TMPFILE=$(mktemp)
curl -fsSL --progress-bar "$DOWNLOAD_URL" -o "$TMPFILE"

# --- Install ---
chmod +x "$TMPFILE"
mv "$TMPFILE" "$INSTALL_DIR/repomind"
echo "Installed: $INSTALL_DIR/repomind"

# --- Ensure PATH ---
if ! echo "$PATH" | tr ':' '\n' | grep -qxF "$INSTALL_DIR"; then
    echo ""
    echo "$INSTALL_DIR is not in your PATH."

    # Detect shell and write to the right rc file
    SHELL_NAME=$(basename "$SHELL")
    RC_FILE=""
    case "$SHELL_NAME" in
        zsh)
            if [ -f "$HOME/.zshrc" ]; then
                RC_FILE="$HOME/.zshrc"
            elif [ -f "$HOME/.zprofile" ]; then
                RC_FILE="$HOME/.zprofile"
            fi
            ;;
        bash)
            if [ -f "$HOME/.bashrc" ]; then
                RC_FILE="$HOME/.bashrc"
            elif [ -f "$HOME/.bash_profile" ]; then
                RC_FILE="$HOME/.bash_profile"
            elif [ -f "$HOME/.profile" ]; then
                RC_FILE="$HOME/.profile"
            fi
            ;;
    esac

    if [ -n "$RC_FILE" ]; then
        echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$RC_FILE"
        echo "Added to $RC_FILE"
    fi

    # Also export for current session
    export PATH="$INSTALL_DIR:$PATH"
    echo "Run this to use repomind now:"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
else
    echo "$INSTALL_DIR is already in PATH"
    # Rehash if needed
    hash -r 2>/dev/null || true
fi

# --- Verify ---
echo ""
"$INSTALL_DIR/repomind" --help 2>&1 | head -3
echo ""
echo "RepoMind installed successfully!"
echo "Next: cd into your project and run 'repomind install'"
