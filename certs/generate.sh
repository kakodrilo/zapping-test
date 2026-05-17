#!/bin/sh
# Genera un certificado autofirmado para desarrollo local.
# Ejecutar desde la RAÍZ del proyecto, no desde dentro de certs/.
#
# Linux / macOS / WSL:
#   sh certs/generate.sh
#
# Git Bash (Windows) — MSYS_NO_PATHCONV evita que -subj sea interpretado como ruta:
#   MSYS_NO_PATHCONV=1 sh certs/generate.sh

set -e
export MSYS_NO_PATHCONV=1

openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout certs/key.pem \
  -out    certs/cert.pem \
  -subj   "/C=AR/ST=Local/L=Local/O=Zapping/CN=localhost" \
  -addext "subjectAltName=IP:127.0.0.1,DNS:localhost"

echo "Certificados generados en certs/"
