#!/bin/sh
set -e

REPO="cjccjj/mdflow"
BIN="mdflow"
DEST="/usr/local/bin"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64)   ARCH="arm64" ;;
    *)       echo "Unsupported arch: $ARCH"; exit 1 ;;
esac

TAG=$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
if [ -z "$TAG" ]; then
    echo "Failed to get latest release tag"
    exit 1
fi

URL="https://github.com/$REPO/releases/download/$TAG/$BIN-$OS-$ARCH"
echo "Installing mdflow $TAG to $DEST/$BIN ..."
curl -sL "$URL" -o "/tmp/$BIN"
chmod +x "/tmp/$BIN"
sudo mv "/tmp/$BIN" "$DEST/$BIN"
echo "Done. Run: mdflow"
