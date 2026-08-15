#!/usr/bin/env sh
# mdflow installer - downloads a release binary, verifies its checksum, and
# installs it to ~/.local/bin.
#
#   curl -fsSL https://raw.githubusercontent.com/cjccjj/mdflow/main/install.sh | sh
set -eu

VERSION="0.1.1"
INSTALL_DIR="${HOME}/.local/bin"
REPO="cjccjj/mdflow"
BASE="https://github.com/${REPO}/releases/download/v${VERSION}"

case "$(uname -s)" in
    Linux)  OS="linux" ;;
    Darwin) OS="macos" ;;
    *)
        echo "error: unsupported OS: $(uname -s)" >&2
        exit 1
        ;;
esac

case "$(uname -m)" in
    x86_64|amd64)  ARCH="x86_64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)
        echo "error: unsupported architecture: $(uname -m)" >&2
        exit 1
        ;;
esac

case "$OS" in
    linux)
        # linux x86_64 and arm64 builds are published.
        ;;
    macos)
        if [ "$ARCH" != "arm64" ]; then
            echo "error: no macOS ${ARCH} build is published; use an arm64 Mac or build from source" >&2
            exit 1
        fi
        ;;
esac

ARCHIVE="mdflow_${VERSION}_${OS}_${ARCH}.tar.gz"
BIN="mdflow"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "Downloading mdflow v${VERSION} (${OS}/${ARCH}) ..."
curl -fsSL --retry 3 --retry-all-errors -o "$TMP/checksums.txt" "${BASE}/checksums.txt"
curl -fsSL --retry 3 --retry-all-errors -o "$TMP/${ARCHIVE}" "${BASE}/${ARCHIVE}"

expected="$(awk -v a="$ARCHIVE" '$2 == a { print $1 }' "$TMP/checksums.txt")"
if [ -z "$expected" ]; then
    echo "error: no checksum found for ${ARCHIVE}" >&2
    exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$TMP/${ARCHIVE}" | awk '{ print $1 }')"
else
    actual="$(shasum -a 256 "$TMP/${ARCHIVE}" | awk '{ print $1 }')"
fi
if [ "$actual" != "$expected" ]; then
    echo "error: checksum mismatch for ${ARCHIVE}" >&2
    exit 1
fi
echo "Checksum OK"

tar -xzf "$TMP/${ARCHIVE}" -C "$TMP"

mkdir -p "$INSTALL_DIR"
install -m 0755 "$TMP/${BIN}" "$INSTALL_DIR/${BIN}"

if "$INSTALL_DIR/${BIN}" --help >/dev/null 2>&1; then
    echo "Installed mdflow v${VERSION} to ${INSTALL_DIR}/${BIN}"
else
    echo "Installed to ${INSTALL_DIR}/${BIN}, but the smoke test failed" >&2
    exit 1
fi

case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *) echo "Note: add ${INSTALL_DIR} to your PATH to run mdflow." ;;
esac
