#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERT_DIR="$SCRIPT_DIR/../listener-certs"
CERT_NAME="default-listener"
DAYS_VALID=36500
COMMON_NAME="localhost"

mkdir -p "$CERT_DIR"

openssl req -x509 \
    -newkey rsa:4096 \
    -keyout "$CERT_DIR/$CERT_NAME.key" \
    -out "$CERT_DIR/$CERT_NAME.crt" \
    -days $DAYS_VALID \
    -nodes \
    -subj "/C=US/ST=California/L=San Francisco/O=WSO2/OU=API Gateway/CN=$COMMON_NAME" \
    -addext "subjectAltName=DNS:localhost,DNS:*.localhost,IP:127.0.0.1"

echo "Certificate and key generated successfully in $CERT_DIR/"
echo "  Certificate: $CERT_DIR/$CERT_NAME.crt"
echo "  Private Key: $CERT_DIR/$CERT_NAME.key"
