#!/usr/bin/env bash
# OUMStudio-Novel isolated runner. Usage:
#   ./run.sh                          # interactive TUI
#   ./run.sh --headless --prompt "..."  # autonomous headless
set -euo pipefail
docker run --rm -it \
  --name oumstudio-novel-run \
  -v oumstudio-config:/root/.ainovel \
  -v oumstudio-workspace:/workspace \
  -w /workspace \
  oumstudio-novel:1.2.0-omni "$@"
