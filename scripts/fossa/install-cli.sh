#!/usr/bin/env bash
# Install a pinned fossa-cli release for linux/amd64 after verifying the
# checksum published next to the release asset.
#
# Usage: scripts/fossa/install-cli.sh <version>      e.g. 3.20.2
#
# The binary goes to $FOSSA_INSTALL_DIR (default: $RUNNER_TEMP/fossa-bin, or
# $HOME/.local/bin outside GitHub Actions). Inside Actions the directory is
# appended to $GITHUB_PATH so later steps find `fossa`.
set -euo pipefail

version="${1:?usage: $0 <fossa-cli version, e.g. 3.20.2>}"
case "$version" in
  [0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "install-cli: version must look like 3.20.2, got '$version'" >&2; exit 2 ;;
esac

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$os/$arch" in
  linux/x86_64) asset="fossa_${version}_linux_amd64.zip" ;;
  *) echo "install-cli: only linux/amd64 is supported (got $os/$arch)" >&2; exit 2 ;;
esac

base="https://github.com/fossas/fossa-cli/releases/download/v${version}"
install_dir="${FOSSA_INSTALL_DIR:-${RUNNER_TEMP:+$RUNNER_TEMP/fossa-bin}}"
install_dir="${install_dir:-$HOME/.local/bin}"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "install-cli: downloading $asset"
curl -fsSL --retry 3 --retry-delay 2 -o "$work/$asset" "$base/$asset"
curl -fsSL --retry 3 --retry-delay 2 -o "$work/$asset.sha256" "$base/$asset.sha256"

# The published file has the form "<sha256> *<asset>"; sha256sum -c accepts it
# when run from the directory holding the asset.
(cd "$work" && sha256sum -c "$asset.sha256")

mkdir -p "$install_dir"
unzip -qo "$work/$asset" fossa -d "$install_dir"
chmod 0755 "$install_dir/fossa"

installed="$("$install_dir/fossa" --version)"
case "$installed" in
  *"version ${version} "*|*"version ${version}") ;;
  *) echo "install-cli: expected fossa-cli $version, got: $installed" >&2; exit 1 ;;
esac
echo "install-cli: $installed -> $install_dir/fossa"

if [ -n "${GITHUB_PATH:-}" ]; then
  echo "$install_dir" >> "$GITHUB_PATH"
fi
