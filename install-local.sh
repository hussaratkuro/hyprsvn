#!/usr/bin/env bash
set -euo pipefail

project_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
build_dir="$(mktemp -d)"
trap 'rm -rf -- "$build_dir"' EXIT

cd "$project_dir"
go build -trimpath -tags novulkan -o "$build_dir/hyprsvn" .
install -Dm755 "$build_dir/hyprsvn" "$HOME/.local/bin/hyprsvn"
install -Dm644 hyprsvn.png "$HOME/.local/share/icons/hyprsvn/hyprsvn.png"
sed "s|__HOME__|$HOME|g" hyprsvn.desktop > "$build_dir/hyprsvn.desktop"
install -Dm644 "$build_dir/hyprsvn.desktop" "$HOME/.local/share/applications/hyprsvn.desktop"

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database "$HOME/.local/share/applications"
fi

echo "Installed hyprsvn in $HOME/.local/bin"
