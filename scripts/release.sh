#!/bin/bash
# Release packaging script for Beryl 7 AI Agent

set -e

VERSION="v16.0"
DIST_DIR="dist"

echo "=== Building Beryl 7 AI Agent ${VERSION} Release Artifacts ==="
mkdir -p ${DIST_DIR}

# 1. Cross-compile ARM64 Linux (GL-MT3600BE / Beryl 7)
echo "Cross-compiling linux/arm64 binary..."
cd go-agent
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ../${DIST_DIR}/beryl7-agent-linux-arm64 ./cmd
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../${DIST_DIR}/beryl7-agent-linux-amd64 ./cmd
cd ..

# 2. Package release archives
cd ${DIST_DIR}
tar -czvf beryl7-agent-${VERSION}-linux-arm64.tar.gz beryl7-agent-linux-arm64
tar -czvf beryl7-agent-${VERSION}-linux-amd64.tar.gz beryl7-agent-linux-amd64

# 3. Generate SHA256 checksums & optional GPG Signature
sha256sum beryl7-agent-${VERSION}-*.tar.gz > SHA256SUMS
if command -v gpg >/dev/null 2>&1; then
    gpg --armor --detach-sign SHA256SUMS 2>/dev/null || echo "GPG Signing skipped (no default private key available)."
fi

echo "=== Release Artifacts Packaged Successfully in ${DIST_DIR}/ ==="
cat SHA256SUMS
