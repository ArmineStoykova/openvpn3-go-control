#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
APP="${PROJECT_DIR}/openvpn3-control"
BIN_DIR="${HOME}/.local/bin"
APP_DIR="${HOME}/.local/share/applications"
LAUNCHER="${BIN_DIR}/openvpn3-control-launcher"
DESKTOP="${APP_DIR}/openvpn3-control.desktop"

if [[ ! -f "${PROJECT_DIR}/go.mod" ]]; then
  echo "Error: run this installer from the cloned project directory." >&2
  exit 1
fi

if [[ ! -x "${APP}" ]]; then
  if ! command -v go >/dev/null 2>&1; then
    echo "Error: executable missing. Install Go and build: go build -o openvpn3-control ." >&2
    exit 1
  fi
  (cd "${PROJECT_DIR}" && go build -o openvpn3-control .)
fi

mkdir -p -- "${BIN_DIR}" "${APP_DIR}"
# %q makes the project path safe to embed in the Bash launcher.
printf '#!/usr/bin/env bash\ncd -- %q || exit 1\nexec ./openvpn3-control\n' "${PROJECT_DIR}" > "${LAUNCHER}"
chmod 755 "${LAUNCHER}"

# Desktop Entry Exec uses its own quoting rules; reject uncommon unsafe home paths.
if [[ "${LAUNCHER}" == *[\"\\\`\$]* ]]; then
  echo "Error: unsupported special character in home directory path." >&2
  exit 1
fi
printf '[Desktop Entry]\nVersion=1.0\nType=Application\nName=OpenVPN 3 Control\nComment=Start local OpenVPN 3 Go control panel\nExec="%s"\nTerminal=true\nIcon=network-vpn\nCategories=Network;Utility;\nStartupNotify=false\n' "${LAUNCHER}" > "${DESKTOP}"
chmod 644 "${DESKTOP}"

echo "Installed desktop launcher: ${DESKTOP}"
echo "Find OpenVPN 3 Control in your Ubuntu applications menu."
echo "It starts the local server; open http://127.0.0.1:8765 in a browser."
echo "Keep the project directory in place, or rerun this installer after moving it."
