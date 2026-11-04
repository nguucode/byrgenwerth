#!/bin/sh
# Builds dist/Byrgenwerth.app, for Apple Silicon and Intel. Usage: scripts/build-mac.sh [version]
set -eu
version="${1:-0.0.0}"
export CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0
app=dist/Byrgenwerth.app
rm -rf dist
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources" dist/tmp

for arch in arm64 amd64; do
	clang_arch=$arch
	[ "$arch" = amd64 ] && clang_arch=x86_64
	GOOS=darwin GOARCH=$arch CC="clang -arch $clang_arch" CGO_CFLAGS="-arch $clang_arch" CGO_LDFLAGS="-arch $clang_arch" \
		go build -ldflags "-s -w -X main.version=$version" -o "dist/tmp/$arch" .
done
lipo -create dist/tmp/arm64 dist/tmp/amd64 -output "$app/Contents/MacOS/Byrgenwerth"

set=dist/tmp/icon.iconset
mkdir -p "$set"
for size in 16 32 128 256 512; do
	sips -z "$size" "$size" icon.png --out "$set/icon_${size}x${size}.png" >/dev/null
done
iconutil -c icns "$set" -o "$app/Contents/Resources/icon.icns"

cat > "$app/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
	<key>CFBundleName</key><string>Byrgenwerth</string>
	<key>CFBundleDisplayName</key><string>Byrgenwerth</string>
	<key>CFBundleIdentifier</key><string>com.nguucode.byrgenwerth</string>
	<key>CFBundleExecutable</key><string>Byrgenwerth</string>
	<key>CFBundleIconFile</key><string>icon</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>$version</string>
	<key>CFBundleVersion</key><string>$version</string>
	<key>LSMinimumSystemVersion</key><string>14.0</string>
	<key>LSUIElement</key><true/>
	<key>NSHighResolutionCapable</key><true/>
	<key>NSAppTransportSecurity</key><dict><key>NSAllowsLocalNetworking</key><true/></dict>
</dict></plist>
PLIST

rm -rf dist/tmp
# Not notarized: an ad-hoc signature is what lets the app run on Apple Silicon.
codesign --force --deep --sign - "$app"
(cd dist && ditto -c -k --keepParent Byrgenwerth.app Byrgenwerth-macos.zip)
echo "Built $app"
