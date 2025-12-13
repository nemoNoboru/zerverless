#!/bin/bash
# Download MicroPython WASM binary

set -e

VERSION="1.23.0"
URL="https://micropython.org/resources/firmware/micropython-${VERSION}.wasm"
DEST="./bin/micropython.wasm"

mkdir -p ./bin

echo "Downloading MicroPython ${VERSION} WASM..."

# MicroPython official doesn't have a direct WASM download
# We'll use the aspect-build version which is WASI-compatible
# Alternative: build from source

# For now, use a community build or build instructions
echo "MicroPython WASM needs to be built from source or obtained from:"
echo "  1. https://github.com/aspect-build/aspect-wasi-python"
echo "  2. Build from https://github.com/nickovs/micropython-wasm"
echo "  3. Build from official MicroPython with WASI port"
echo ""
echo "To build from source:"
echo "  git clone https://github.com/micropython/micropython.git"
echo "  cd micropython/ports/webassembly"
echo "  make"
echo ""
echo "Or download a pre-built binary and place at: ${DEST}"

