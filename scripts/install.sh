#!/usr/bin/env bash
# Usage: curl -fsSL https://raw.githubusercontent.com/coolqoo/better-apigate/main/scripts/install.sh | bash
set -euo pipefail

apigate_repo="${APIGATE_REPOSITORY:-coolqoo/better-apigate}"
apigate_install_dir="${INSTALL_DIR:-/usr/local/bin}"
apigate_version="${VERSION:-}"
apigate_tmp=""
trap 'if [[ -n "$apigate_tmp" ]]; then rm -rf -- "$apigate_tmp"; fi' EXIT

apigate_os="$(uname -s | tr '[:upper:]' '[:lower:]')"
apigate_arch="$(uname -m)"
case "$apigate_os" in
  linux|darwin) ;;
  mingw*|msys*|cygwin*) apigate_os=windows ;;
  *) echo "Unsupported operating system: $apigate_os" >&2; exit 1 ;;
esac
case "$apigate_arch" in
  x86_64|amd64) apigate_arch=amd64 ;;
  aarch64|arm64) apigate_arch=arm64 ;;
  *) echo "Unsupported architecture: $apigate_arch" >&2; exit 1 ;;
esac
if [[ "$apigate_os" == windows && "$apigate_arch" != amd64 ]]; then
  echo "Windows releases currently support amd64." >&2; exit 1
fi

apigate_gh=false
if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then apigate_gh=true; fi
if [[ -z "$apigate_version" ]]; then
  if [[ "$apigate_gh" == true ]]; then
    apigate_version="$(gh release view --repo "$apigate_repo" --json tagName --jq .tagName)"
  else
    apigate_version="$(curl -fsSL "https://api.github.com/repos/$apigate_repo/releases/latest" | sed -nE 's/.*"tag_name": *"([^"]+)".*/\1/p')"
  fi
fi
if [[ ! "$apigate_version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-+][a-zA-Z0-9.-]+)?$ ]]; then
  echo "Cannot resolve a release. Set VERSION to a published version such as v1.0.0." >&2; exit 1
fi

apigate_platform="$apigate_os-$apigate_arch"
apigate_extension=tar.gz
apigate_suffix=""
if [[ "$apigate_os" == windows ]]; then apigate_extension=zip; apigate_suffix=.exe; fi
apigate_archive="better-apigate-$apigate_platform.$apigate_extension"
apigate_tmp="$(mktemp -d)"
if [[ "$apigate_gh" == true ]]; then
  gh release download "$apigate_version" --repo "$apigate_repo" --dir "$apigate_tmp" --pattern "$apigate_archive" --pattern checksums.txt
else
  apigate_download="https://github.com/$apigate_repo/releases/download/$apigate_version"
  curl -fsSL "$apigate_download/$apigate_archive" -o "$apigate_tmp/$apigate_archive"
  curl -fsSL "$apigate_download/checksums.txt" -o "$apigate_tmp/checksums.txt"
fi
cd "$apigate_tmp"
apigate_expected="$(awk -v target="$apigate_archive" '$2 == target || $2 == "*" target {print $1}' checksums.txt)"
if [[ ! "$apigate_expected" =~ ^[[:xdigit:]]{64}$ ]]; then
  echo "Release checksum is missing or ambiguous." >&2; exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  apigate_actual="$(sha256sum "$apigate_archive" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  apigate_actual="$(shasum -a 256 "$apigate_archive" | awk '{print $1}')"
else
  echo "Install sha256sum or shasum before installing better-apigate." >&2; exit 1
fi
if [[ "$apigate_actual" != "$apigate_expected" ]]; then
  echo "Release checksum mismatch. Installation stopped." >&2; exit 1
fi

# CI archives contain the platform name; GoReleaser archives contain better-apigate.
apigate_binary="better-apigate-$apigate_platform$apigate_suffix"
if [[ "$apigate_extension" == zip ]]; then
  apigate_entries="$(unzip -Z1 "$apigate_archive")"
else
  apigate_entries="$(tar -tzf "$apigate_archive")"
fi
if ! printf '%s\n' "$apigate_entries" | awk -v target="$apigate_binary" '$0==target {found=1} END {exit !found}'; then
  apigate_binary="better-apigate$apigate_suffix"
fi
if ! printf '%s\n' "$apigate_entries" | awk -v target="$apigate_binary" '$0==target {found=1} END {exit !found}'; then
  echo "Release archive does not contain the expected executable." >&2; exit 1
fi
if [[ "$apigate_extension" == zip ]]; then
  unzip -p "$apigate_archive" "$apigate_binary" > "$apigate_binary"
else
  tar -xzf "$apigate_archive" -- "$apigate_binary"
fi
apigate_destination="$apigate_install_dir/better-apigate$apigate_suffix"
if [[ -w "$apigate_install_dir" ]] || { [[ ! -e "$apigate_install_dir" ]] && [[ -w "$(dirname "$apigate_install_dir")" ]]; }; then
  mkdir -p "$apigate_install_dir"
  install -m 0755 "$apigate_binary" "$apigate_destination"
else
  sudo install -d "$apigate_install_dir"
  sudo install -m 0755 "$apigate_binary" "$apigate_destination"
fi
printf 'Installed better-apigate %s at %s\nConfigure PostgreSQL, Redis and deployment secrets before running better-apigate serve.\n' "$apigate_version" "$apigate_destination"
