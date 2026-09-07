#!/bin/sh

set -eu

if [ "$(uname -s)" != "Darwin" ]; then
  echo "This script only supports macOS." >&2
  exit 1
fi

for tool in make go pnpm fyne osascript ditto; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "Missing required command: $tool" >&2
    exit 1
  fi
done

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
desktop_dir=$(osascript -e 'POSIX path of (path to desktop folder)')
desktop_dir=${desktop_dir%/}
source_app="$repo_dir/Rooster.app"
target_app="$desktop_dir/Rooster.app"

echo "Building Rooster.app..."
make -C "$repo_dir" package-darwin

if [ ! -d "$source_app" ]; then
  echo "Package not found: $source_app" >&2
  exit 1
fi

staging_dir=$(mktemp -d "$desktop_dir/.rooster-install.XXXXXX")
case "$staging_dir" in
  "$desktop_dir"/.rooster-install.*) ;;
  *)
    echo "Unexpected staging directory: $staging_dir" >&2
    exit 1
    ;;
esac

staged_app="$staging_dir/Rooster.app"
previous_app="$staging_dir/Previous-Rooster.app"
cleanup() {
  if { [ ! -e "$target_app" ] && [ ! -L "$target_app" ]; } && \
    { [ -e "$previous_app" ] || [ -L "$previous_app" ]; }; then
    mv "$previous_app" "$target_app" || true
  fi
  rm -rf -- "${staging_dir:?}"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

ditto "$source_app" "$staged_app"

if [ -e "$target_app" ] || [ -L "$target_app" ]; then
  mv "$target_app" "$previous_app"
fi

if ! mv "$staged_app" "$target_app"; then
  echo "Installation failed; the previous desktop app was restored." >&2
  exit 1
fi

echo "Installed: $target_app"
