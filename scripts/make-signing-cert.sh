#!/bin/sh
# Creates the self-signed certificate that release builds of the Mac app are
# signed with, and stores it as the GitHub secrets MAC_CERT_P12 and
# MAC_CERT_PASSWORD. Run once on a Mac, with gh logged in. The key never touches
# the disk outside a temp folder that is removed on exit.
#
# Running it again makes a new certificate: Mac users then grant Screen
# Recording once more after the next update, so only do that if the old one leaked.
set -eu
repo=nguucode/byrgenwerth
name="Byrgenwerth Code Signing"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cat > "$tmp/cfg" <<EOF
[req]
distinguished_name=dn
x509_extensions=ext
prompt=no
[dn]
CN=$name
[ext]
basicConstraints=critical,CA:false
keyUsage=critical,digitalSignature
extendedKeyUsage=critical,codeSigning
EOF
# LibreSSL from macOS writes a .p12 that `security import` reads; OpenSSL 3 would need -legacy.
/usr/bin/openssl req -x509 -newkey rsa:2048 -nodes -days 7300 -config "$tmp/cfg" \
	-keyout "$tmp/key.pem" -out "$tmp/cert.pem" 2>/dev/null
pass=$(/usr/bin/openssl rand -hex 24)
/usr/bin/openssl pkcs12 -export -inkey "$tmp/key.pem" -in "$tmp/cert.pem" \
	-out "$tmp/cert.p12" -passout "pass:$pass"

base64 -i "$tmp/cert.p12" | gh secret set MAC_CERT_P12 --repo "$repo"
printf %s "$pass" | gh secret set MAC_CERT_PASSWORD --repo "$repo"
echo "Saved MAC_CERT_P12 and MAC_CERT_PASSWORD to $repo."
echo "Certificate SHA-1: $(/usr/bin/openssl x509 -in "$tmp/cert.pem" -noout -fingerprint -sha1 | cut -d= -f2)"
