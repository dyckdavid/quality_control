#!/usr/bin/env bash
# Builds a universal (Apple Silicon + Intel) Quality Control.app and wraps it in a drag-to-Applications DMG.
# Usage: desktop/packaging/macos/build-dmg.sh [output-dir]
# Set QC_SIGN_IDENTITY to a "Developer ID Application" identity to sign for distribution; otherwise it is ad-hoc signed.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
desktop="$(cd "$here/../.." && pwd)"
out="${1:-$desktop/target/dist}"
version="$(sed -n 's/^version = "\(.*\)"/\1/p' "$desktop/Cargo.toml" | head -1)"
app_name="Quality Control"
target_dir="${CARGO_TARGET_DIR:-$desktop/target}"
work="$target_dir/macos-bundle"

rustup target add aarch64-apple-darwin x86_64-apple-darwin >/dev/null
for target in aarch64-apple-darwin x86_64-apple-darwin; do
  MACOSX_DEPLOYMENT_TARGET=11.0 cargo build --release --locked --manifest-path "$desktop/Cargo.toml" --target "$target" --bin qc-desktop
done

rm -rf "$work"
app="$work/$app_name.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
lipo -create \
  "$target_dir/aarch64-apple-darwin/release/qc-desktop" \
  "$target_dir/x86_64-apple-darwin/release/qc-desktop" \
  -output "$app/Contents/MacOS/qc-desktop"

iconset="$work/AppIcon.iconset"
mkdir -p "$iconset"
for size in 16 32 128 256 512; do
  sips -z "$size" "$size" "$desktop/assets/icon.png" --out "$iconset/icon_${size}x${size}.png" >/dev/null
  double=$((size * 2))
  sips -z "$double" "$double" "$desktop/assets/icon.png" --out "$iconset/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$iconset" -o "$app/Contents/Resources/AppIcon.icns"

sed "s/__VERSION__/$version/g" "$here/Info.plist" > "$app/Contents/Info.plist"

identity="${QC_SIGN_IDENTITY:--}"
codesign --force --options runtime --timestamp=none --sign "$identity" "$app"

mkdir -p "$out"
dmg="$out/QualityControl-$version-macos.dmg"
stage="$work/dmg"
mkdir -p "$stage"
cp -R "$app" "$stage/"
ln -s /Applications "$stage/Applications"
rm -f "$dmg"
hdiutil create -volname "$app_name" -srcfolder "$stage" -fs HFS+ -format UDZO -ov "$dmg" >/dev/null
echo "$dmg"
