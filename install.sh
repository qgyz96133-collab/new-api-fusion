#!/usr/bin/env bash

set -uo pipefail

# Qoder CLI Installation Script
# Responsibilities (script layer):
#   - OS/Arch/CPU feature detection
#   - Manifest fetch + binary download + SHA256 verification + extraction
# Post-extraction, delegates to the binary's `install` subcommand for:
#   - Versioned binary placement, entry-point creation, PATH config,
#     marker writing, verification, cleanup, and telemetry.

BASE_URL="https://qoder-ide.oss-accelerate.aliyuncs.com/qodercli"
FORCE=0

export QODER_CLI_INSTALL=1

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

usage() {
  cat <<EOF >&2
Qoder CLI Installation Script

USAGE:
  curl -fsSL ${BASE_URL}/install.sh | bash
  curl -fsSL ${BASE_URL}/install.sh | bash -s -- [OPTIONS]

OPTIONS:
  --force                Force overwrite existing installation
  -h, --help            Show this help message
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --force) FORCE=1; shift ;;
    *) echo "Error: Unknown option $1" >&2; usage; exit 1 ;;
  esac
done

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: $1 is required but not installed" >&2
    exit 1
  fi
}

fatal_error() {
  echo "" >&2
  echo "Fatal Error: $1" >&2
  if [[ $# -gt 1 ]]; then
    shift
    echo "" >&2
    for suggestion in "$@"; do
      echo "  - $suggestion" >&2
    done
  fi
  echo "" >&2
  exit 1
}

download() {
  local url="$1" output="$2"
  local max_retries=3 retry_count=0

  while [[ $retry_count -lt $max_retries ]]; do
    [[ $retry_count -gt 0 ]] && sleep 2

    if command -v curl >/dev/null 2>&1; then
      if curl -fsSL --retry 2 --connect-timeout 30 --max-time 300 \
        -H "User-Agent: qodercli-installer/curl-bash (https://qoder.com)" \
        "$url" -o "$output"; then
        return 0
      fi
    elif command -v wget >/dev/null 2>&1; then
      if wget -q --tries=2 --connect-timeout=30 --read-timeout=300 \
        --user-agent="qodercli-installer/curl-bash (https://qoder.com)" \
        "$url" -O "$output"; then
        return 0
      fi
    else
      echo "Error: Neither curl nor wget is available" >&2
      exit 1
    fi

    rm -f "$output" 2>/dev/null || true
    retry_count=$((retry_count + 1))
  done

  return 1
}

# ---------------------------------------------------------------------------
# Platform detection
# ---------------------------------------------------------------------------

detect_os_arch() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"

  # macOS: detect Apple Silicon even under Rosetta 2
  if [[ "$os" == "darwin" && "$arch" == "x86_64" ]]; then
    if sysctl -n hw.optional.arm64 2>/dev/null | grep -q '1'; then
      arch="arm64"
      echo "==> Detected Apple Silicon (Rosetta); selecting arm64 binary" >&2
    fi
  fi

  [[ "$arch" == "aarch64" ]] && arch="arm64"
  [[ "$arch" == "x86_64" ]] && arch="amd64"

  echo "$os $arch"
}

BUN_OPTIMIZED_X64_REQUIRED_CPU_FLAGS=(sse4_2 popcnt avx avx2 bmi1 bmi2 fma)

linux_cpu_has_flags() {
  local cpuinfo_path="${QODERCLI_CPUINFO_PATH:-/proc/cpuinfo}"
  [[ -r "$cpuinfo_path" ]] || return 1

  local flags_line
  flags_line="$(
    grep -m1 -E '^flags[[:space:]]*:' "$cpuinfo_path" 2>/dev/null || true
  )"
  [[ -n "$flags_line" ]] || return 1

  local flag
  for flag in "$@"; do
    if ! printf '%s\n' "$flags_line" |
      grep -Eq "(^|[[:space:]])${flag}($|[[:space:]])"; then
      return 1
    fi
  done

  return 0
}

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------

TMP_DIR=""
cleanup() {
  if [[ -n "$TMP_DIR" && -d "$TMP_DIR" ]]; then
    rm -rf "$TMP_DIR" 2>/dev/null || true
  fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

main() {
  # Windows detection - exit immediately
  local os_name
  os_name="$(uname -s | tr '[:upper:]' '[:lower:]')"
  if [[ "$os_name" == *"mingw"* || "$os_name" == *"cygwin"* || "$os_name" == *"msys"* ]]; then
    echo "Windows detected. Use PowerShell installer or npm instead." >&2
    echo "  irm ${BASE_URL}/install.ps1 | iex" >&2
    echo "  npm install -g @qoder-ai/qodercli" >&2
    exit 1
  fi

  TMP_DIR="$(mktemp -d -t qoder-install.XXXXXX)"
  trap cleanup EXIT INT TERM

  local os arch
  read -r os arch < <(detect_os_arch)

  # Bun's optimized Linux x64 target needs more than AVX2. If any required
  # CPU flag is missing or cannot be verified, prefer the compatible baseline.
  if [[ "$os" == "linux" && "$arch" == "amd64" ]]; then
    if ! linux_cpu_has_flags "${BUN_OPTIMIZED_X64_REQUIRED_CPU_FLAGS[@]}"; then
      echo "==> CPU lacks Bun optimized x64 requirements; selecting baseline binary"
      arch="amd64-baseline"
    fi
  fi

  echo "==> Platform: $os/$arch"

  # -----------------------------------------------------------------------
  # Fetch manifest
  # -----------------------------------------------------------------------
  local manifest_url="$BASE_URL/channels/manifest.json"
  local manifest_file="$TMP_DIR/manifest.json"

  echo "==> Fetching release information..."
  if ! download "$manifest_url" "$manifest_file"; then
    fatal_error "Failed to download release manifest from $BASE_URL"
  fi

  local manifest_json
  manifest_json="$(cat "$manifest_file")"

  # Extract version
  local version
  version=$(printf '%s' "$manifest_json" | sed -n 's/.*"latest"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
  [[ -z "$version" ]] && fatal_error "Cannot parse version from manifest"

  echo "==> Latest version: $version"

  # Find matching platform entry
  local entry
  entry=$(printf '%s' "$manifest_json" | tr -d '\n\r\t ' | sed 's/},{/}\n{/g' | grep -F "\"os\":\"$os\"" | grep -F "\"arch\":\"$arch\"" | head -n1)
  [[ -z "$entry" ]] && fatal_error "No binary available for $os/$arch"

  # Extract URL and checksum
  local download_url checksum
  download_url=$(printf '%s' "$entry" | sed -n 's/.*"url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
  checksum=$(printf '%s' "$entry" | sed -n 's/.*"sha256"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
  [[ -z "$download_url" ]] && fatal_error "Missing download URL in manifest for $os/$arch"

  # -----------------------------------------------------------------------
  # Download
  # -----------------------------------------------------------------------
  echo "==> Downloading Qoder CLI $version..."
  local archive_filename archive_file
  archive_filename=$(basename "$download_url")
  archive_file="$TMP_DIR/$archive_filename"

  # Check extraction tool
  if [[ "$archive_filename" =~ \.zip$ ]]; then
    require_cmd unzip
  elif [[ "$archive_filename" =~ \.(tar\.gz|tgz)$ ]]; then
    require_cmd tar
  else
    fatal_error "Unsupported archive format: $archive_filename"
  fi

  if ! download "$download_url" "$archive_file"; then
    fatal_error "Failed to download binary from $download_url"
  fi

  # Size sanity check
  local file_size
  file_size=$(wc -c < "$archive_file" 2>/dev/null || echo "0")
  [[ "$file_size" -lt 1024 ]] && fatal_error "Download too small ($file_size bytes), likely corrupted"

  # -----------------------------------------------------------------------
  # Verify checksum
  # -----------------------------------------------------------------------
  if [[ -n "$checksum" ]]; then
    echo "==> Verifying checksum..."
    local actual_checksum=""
    if command -v shasum >/dev/null 2>&1; then
      actual_checksum=$(shasum -a 256 "$archive_file" | cut -d' ' -f1)
    elif command -v sha256sum >/dev/null 2>&1; then
      actual_checksum=$(sha256sum "$archive_file" | cut -d' ' -f1)
    fi

    if [[ -n "$actual_checksum" && "$actual_checksum" != "$checksum" ]]; then
      fatal_error "Checksum mismatch" \
        "Expected: $checksum" \
        "Actual:   $actual_checksum"
    fi
    echo "==> Checksum verified"
  fi

  # -----------------------------------------------------------------------
  # Extract
  # -----------------------------------------------------------------------
  echo "==> Extracting..."
  local extract_dir="$TMP_DIR/extract"
  mkdir -p "$extract_dir"

  if [[ "$archive_filename" =~ \.zip$ ]]; then
    unzip -q "$archive_file" -d "$extract_dir" || fatal_error "Failed to extract zip"
  else
    tar -xzf "$archive_file" -C "$extract_dir" || fatal_error "Failed to extract tar.gz"
  fi

  local bin_name="qodercli"
  [[ ! -f "$extract_dir/$bin_name" ]] && fatal_error "Binary $bin_name not found in archive"
  chmod +x "$extract_dir/$bin_name"

  # -----------------------------------------------------------------------
  # Delegate to binary's install subcommand
  # -----------------------------------------------------------------------
  echo "==> Installing..."
  if ! "$extract_dir/$bin_name" install --force; then
    fatal_error "Install subcommand failed"
  fi
}

main "$@"
