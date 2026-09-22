#!/bin/sh
set -e

# PureSend Universal Installer for Linux & macOS
# Usage: curl -fsSL https://raw.githubusercontent.com/Baaaki/PureSend/main/install.sh | sh

REPO="Baaaki/PureSend"
BINARY="puresend"

# The minisign public key releases are signed with: the same key as the
# FT_UPDATE_KEY repository variable. Empty until release signing is set up
# (docs/DEPLOYMENT.md); the SHA-256 check below runs either way.
PUBKEY=""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info() {
    printf "${BLUE}==>${NC} %s\n" "$1"
}

success() {
    printf "${GREEN}==>${NC} %s\n" "$1"
}

error() {
    printf "${RED}Hata:${NC} %s\n" "$1" >&2
    exit 1
}

# 1. Detect OS
OS="$(uname -s)"
case "$OS" in
    Linux)
        OS_TAG="linux"
        ;;
    Darwin)
        OS_TAG="macOS"
        ;;
    *)
        error "Desteklenmeyen isletim sistemi: $OS. PureSend Linux ve macOS desteklemektedir."
        ;;
esac

# 2. Detect Architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64)
        ARCH_TAG="x86_64"
        ;;
    arm64|aarch64)
        if [ "$OS_TAG" = "macOS" ]; then
            ARCH_TAG="arm64"
        else
            error "ARM64 Linux icin onceden derlenmis ikili dosya bulunmuyor. Kaynak koddan derleyebilirsiniz: git clone https://github.com/$REPO.git && cd PureSend && go build -o puresend ./cmd/client (sunucu adresini -server ile vermen gerekir)"
        fi
        ;;
    *)
        error "Desteklenmeyen mimari: $ARCH"
        ;;
esac

info "Sistem tespiti: $OS_TAG ($ARCH_TAG)"

# 3. Find the latest release. The API allows 60 requests an hour per
# address, which a shared address runs out of, so the redirect behind
# /releases/latest is asked next. A guessed version is not an option: it
# would quietly install an old release.
info "En son surum kontrol ediliyor..."
RELEASE_JSON=$(curl -fsS "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null) || true
LATEST_TAG=$(printf "%s" "$RELEASE_JSON" | grep '"tag_name":' | head -n 1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
    LATEST_TAG=$(curl -fsS -o /dev/null -w '%{redirect_url}' "https://github.com/$REPO/releases/latest" 2>/dev/null |
        sed -n 's#.*/releases/tag/##p') || true
fi

case "$LATEST_TAG" in
    v[0-9]*) ;;
    *) error "En son surum bulunamadi (GitHub'a ulasilamadi ya da istek siniri doldu). Biraz sonra tekrar deneyin." ;;
esac

VERSION="${LATEST_TAG#v}"
ARCHIVE_NAME="puresend_${VERSION}_${OS_TAG}_${ARCH_TAG}.tar.gz"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_TAG/$ARCHIVE_NAME"

info "En son surum indiriliyor: $LATEST_TAG ($ARCHIVE_NAME)..."

TMP_DIR="$(mktemp -d)"
cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$ARCHIVE_NAME" || error "Indirme basarisiz oldu: $DOWNLOAD_URL"

# Nothing is installed unchecked: no checksum file, no install.
info "Dosya butunlugu dogrulaniyor..."
BASE_URL="https://github.com/$REPO/releases/download/$LATEST_TAG"
curl -fsSL "$BASE_URL/checksums.txt" -o "$TMP_DIR/checksums.txt" ||
    error "checksums.txt indirilemedi; dogrulanamayan bir dosya kurulmayacak."

if [ -n "$PUBKEY" ]; then
    if command -v minisign >/dev/null 2>&1; then
        curl -fsSL "$BASE_URL/checksums.txt.minisig" -o "$TMP_DIR/checksums.txt.minisig" ||
            error "Imza dosyasi (checksums.txt.minisig) indirilemedi."
        minisign -Vq -P "$PUBKEY" -m "$TMP_DIR/checksums.txt" ||
            error "Guvenlik hatasi: checksums.txt imzasi gecersiz!"
        success "Imza dogrulandi."
    else
        printf "Not: minisign kurulu degil; imza denetimi atlaniyor, SHA-256 ozeti yine denetleniyor.\n"
    fi
fi

EXPECTED_SHA=$(awk -v f="$ARCHIVE_NAME" '$2 == f || $2 == "*" f { print $1 }' "$TMP_DIR/checksums.txt")
[ -n "$EXPECTED_SHA" ] || error "checksums.txt icinde $ARCHIVE_NAME bulunamadi."
if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL_SHA=$(sha256sum "$TMP_DIR/$ARCHIVE_NAME" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
    ACTUAL_SHA=$(shasum -a 256 "$TMP_DIR/$ARCHIVE_NAME" | awk '{ print $1 }')
else
    error "SHA-256 hesaplayacak bir arac (sha256sum ya da shasum) bulunamadi."
fi
[ "$ACTUAL_SHA" = "$EXPECTED_SHA" ] ||
    error "Guvenlik hatasi: indirilen dosyanin SHA-256 ozeti ($ACTUAL_SHA) beklenenle ($EXPECTED_SHA) eslesmiyor!"
success "SHA-256 ozeti dogrulandi."

info "Arsiv aciliyor..."
tar -xzf "$TMP_DIR/$ARCHIVE_NAME" -C "$TMP_DIR"

if [ ! -f "$TMP_DIR/$BINARY" ]; then
    error "Arsiv icinde $BINARY ikili dosyasi bulunamadi."
fi

# 4. Determine installation target
TARGET_DIR="/usr/local/bin"
USE_SUDO=0

if [ ! -w "$TARGET_DIR" ]; then
    if command -v sudo >/dev/null 2>&1 && [ -t 0 ]; then
        USE_SUDO=1
    else
        TARGET_DIR="$HOME/.local/bin"
        mkdir -p "$TARGET_DIR"
    fi
fi

info "Kurulum yapiliyor -> $TARGET_DIR/$BINARY"

if [ "$USE_SUDO" -eq 1 ]; then
    sudo install -m 755 "$TMP_DIR/$BINARY" "$TARGET_DIR/$BINARY"
else
    install -m 755 "$TMP_DIR/$BINARY" "$TARGET_DIR/$BINARY" 2>/dev/null || {
        cp "$TMP_DIR/$BINARY" "$TARGET_DIR/$BINARY"
        chmod +x "$TARGET_DIR/$BINARY"
    }
fi

# 5. macOS Quarantine removal if Darwin
if [ "$OS_TAG" = "macOS" ]; then
    xattr -d com.apple.quarantine "$TARGET_DIR/$BINARY" 2>/dev/null || true
fi

success "PureSend ($LATEST_TAG) basariyla kuruldu!"

# PATH check
case ":$PATH:" in
    *":$TARGET_DIR:"*) ;;
    *)
        printf "\n${RED}Not:${NC} $TARGET_DIR dizini \$PATH ortam degiskeninizde bulunmuyor.\n"
        printf "Kabuk profilinize (~/.bashrc veya ~/.zshrc) sunu ekleyin:\n"
        printf "  export PATH=\"\$PATH:$TARGET_DIR\"\n\n"
        ;;
esac

printf "\nKullanim:\n"
printf "  Arayuzu acmak icin:   puresend\n"
printf "  Dosya gondermek icin: puresend -send <dosya_veya_klasor>\n"
printf "  Dosya almak icin:     puresend -receive <kod>\n\n"
