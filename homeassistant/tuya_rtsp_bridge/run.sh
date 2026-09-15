#!/bin/bash
# Fallback entry when bashio is absent (local docker test).
set -euo pipefail
echo "[INFO] run.sh started" >&2

export TUYA_BRIDGE_ROOT="${TUYA_BRIDGE_ROOT:-/app}"
export PYTHONPATH="${TUYA_BRIDGE_ROOT}/src${PYTHONPATH:+:$PYTHONPATH}"
export PYTHONUNBUFFERED=1
export XDG_DATA_HOME="${XDG_DATA_HOME:-/data}"
export XDG_CONFIG_HOME="${XDG_CONFIG_HOME:-/config}"
export PATH="/app/bin:${PATH}"

mkdir -p "${XDG_DATA_HOME}/tuya-rtsp-bridge" "${XDG_CONFIG_HOME}/tuya-rtsp-bridge"
cd "${XDG_DATA_HOME}/tuya-rtsp-bridge"

if command -v bashio >/dev/null 2>&1; then
  bashio::log.info "Tuya RTSP Bridge starting (API :8787, RTSP :8554)" || true
  else
  echo "[INFO] Tuya RTSP Bridge starting (API :8787, RTSP :8554)"
fi

echo "[INFO] starting Python server" >&2
exec python3 -u "${TUYA_BRIDGE_ROOT}/src/server.py"
