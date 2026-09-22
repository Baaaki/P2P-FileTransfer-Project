#!/usr/bin/env bash
set -euo pipefail

# Directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

VERSION="${1:-2.0.0}"
ARCH="${2:-amd64}"
DEB_NAME="puresend_${VERSION}_${ARCH}.deb"
OUTPUT_DIR="${ROOT_DIR}/bin"
OUTPUT_FILE="${OUTPUT_DIR}/${DEB_NAME}"

mkdir -p "${OUTPUT_DIR}"

SERVER_ADDR="/dns4/rendezvous.madebybaki.com/tcp/443/tls/ws/p2p/12D3KooWJdXaT1FN4UGLCrrTpdqvpo7cqrJZK6tHvbUPQbJ6APtK"
SERVER_LIST="https://puresend.madebybaki.com/server.txt"

# If binary already exists and no rebuild requested, we can use it or rebuild:
if [ ! -f "${OUTPUT_DIR}/puresend" ]; then
  echo "==> Linux binary derleniyor (${ARCH})..."
  cd "${ROOT_DIR}"
  CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" go build -trimpath -ldflags "-s -w \
    -X main.defaultServer=${SERVER_ADDR} \
    -X main.defaultServerList=${SERVER_LIST} \
    -X main.version=v${VERSION} \
    -X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown') \
    -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o "${OUTPUT_DIR}/puresend" ./cmd/client
fi

echo "==> .deb paket yapısı hazırlanıyor..."
BUILD_DIR="${ROOT_DIR}/.deb_staging_${ARCH}"
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}/DEBIAN"
mkdir -p "${BUILD_DIR}/usr/bin"
mkdir -p "${BUILD_DIR}/usr/share/applications"
mkdir -p "${BUILD_DIR}/usr/share/icons/hicolor/scalable/apps"
trap 'rm -rf "${BUILD_DIR}"' EXIT

chmod 755 "${BUILD_DIR}"
chmod 755 "${BUILD_DIR}/DEBIAN"

# Copy binary
cp "${OUTPUT_DIR}/puresend" "${BUILD_DIR}/usr/bin/puresend"
chmod 755 "${BUILD_DIR}/usr/bin/puresend"

# Copy desktop launcher & icon
cp "${ROOT_DIR}/packaging/puresend.desktop" "${BUILD_DIR}/usr/share/applications/puresend.desktop"
chmod 644 "${BUILD_DIR}/usr/share/applications/puresend.desktop"

cp "${ROOT_DIR}/packaging/puresend.svg" "${BUILD_DIR}/usr/share/icons/hicolor/scalable/apps/puresend.svg"
chmod 644 "${BUILD_DIR}/usr/share/icons/hicolor/scalable/apps/puresend.svg"

# Create DEBIAN/control
cat <<EOF > "${BUILD_DIR}/DEBIAN/control"
Package: puresend
Version: ${VERSION}
Section: utils
Priority: optional
Architecture: ${ARCH}
Maintainer: Baki <contact@madebybaki.com>
Homepage: https://github.com/Baaaki/PureSend
Description: Guvenli, sifreli, dogrudan P2P dosya transfer araci
 PureSend, iki cihaz arasinda araci sunucuya dosya kaydetmeden,
 uctan uca sifreleme (PAKE/SPAKE2) ile dosya ve klasor transferi yapmayi saglar.
EOF
chmod 644 "${BUILD_DIR}/DEBIAN/control"

echo "==> dpkg-deb ile .deb paketi üretiliyor..."
dpkg-deb --build --root-owner-group "${BUILD_DIR}" "${OUTPUT_FILE}"

echo "==> Paket başarıyla oluşturuldu: ${OUTPUT_FILE}"
ls -lh "${OUTPUT_FILE}"
