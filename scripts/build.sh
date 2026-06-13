#!/bin/bash
# AI Monitor Build Script (WSL/MinGW)
set -e

echo "========================================"
echo " Building AI Monitor v1.0.0"
echo "========================================"

GOCMD="/mnt/c/Program Files/Go/bin/go.exe"
MINGW="/mnt/c/msys64/mingw64/bin"
OUTPUT="AI-Monitor.exe"

export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=1
export CC="$MINGW/x86_64-w64-mingw32-gcc.exe"
export CXX="$MINGW/x86_64-w64-mingw32-g++.exe"
export PATH="$MINGW:$PATH"

echo ""
echo "Compiling..."
"$GOCMD" build \
    -ldflags="-H windowsgui -s -w" \
    -trimpath \
    -o "$OUTPUT" \
    .

echo ""
echo "Build successful! Output: $OUTPUT"
ls -lh "$OUTPUT"
echo ""
echo "Done."
