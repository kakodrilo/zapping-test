#!/bin/sh
# Generates a self-signed certificate for localhost development.
#
# Usage:
#   Git Bash (Windows): MSYS_NO_PATHCONV=1 sh certs/generate.sh
#   Linux / macOS / WSL: sh certs/generate.sh

set -e
# Disable MSYS/Git Bash automatic path conversion for the -subj argument
export MSYS_NO_PATHCONV=1

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout "$SCRIPT_DIR/key.pem" \
  -out    "$SCRIPT_DIR/cert.pem" \
  -subj   "/C=AR/ST=Local/L=Local/O=Zapping/CN=localhost" \
  -addext "subjectAltName=IP:127.0.0.1,DNS:localhost"

echo "Certificates generated in certs/"
