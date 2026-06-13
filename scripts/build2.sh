#!/bin/bash
set -e
cd /mnt/d/ai_code/ai_monitor

# Install Go in WSL if not present
if ! command -v go &>/dev/null; then
    echo "Installing Go in WSL..."
    wget -q https://go.dev/dl/go1.24.2.linux-amd64.tar.gz -O /tmp/go.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
fi

export PATH="/usr/local/go/bin:$PATH"
echo "Go: $(go version)"

# Install MinGW cross-compiler in WSL
if ! command -v x86_64-w64-mingw32-gcc &>/dev/null; then
    echo "Installing MinGW cross-compiler..."
    sudo apt-get update -qq
    sudo apt-get install -y -qq gcc-mingw-w64-x86-64 2>&1 | tail -3
fi

export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=1
export CC=x86_64-w64-mingw32-gcc

echo "CGO_ENABLED=$CGO_ENABLED CC=$CC"

# Step 1: mod tidy
go mod tidy 2>&1

echo ""
echo "=== Building DEBUG ==="
go build -ldflags="-s -w" -trimpath -o AI-Monitor-DEBUG.exe . 2>&1

echo ""
echo "=== Building RELEASE ==="
go build -ldflags="-H windowsgui -s -w" -trimpath -o AI-Monitor.exe . 2>&1

echo ""
echo "=== Results ==="
ls -lh /mnt/d/ai_code/ai_monitor/AI-Monitor*.exe 2>/dev/null
echo "Done."
