#!/usr/bin/env bash

set -euo pipefail

cert_dir="${CERT_DIR:-/certs}"
password="${KAFKA_CERT_PASSWORD:-changeit}"
broker_host="${KAFKA_BROKER_HOST:-kafka}"

mkdir -p "${cert_dir}"

if [[ -f "${cert_dir}/kafka.keystore.jks" && -f "${cert_dir}/kafka.truststore.jks" && -f "${cert_dir}/ca.crt" ]]; then
  echo "Kafka TLS assets already exist in ${cert_dir}"
  exit 0
fi

rm -f \
  "${cert_dir}/ca.crt" \
  "${cert_dir}/ca.key" \
  "${cert_dir}/ca.srl" \
  "${cert_dir}/kafka.keystore.jks" \
  "${cert_dir}/kafka.truststore.jks" \
  "${cert_dir}/broker.csr" \
  "${cert_dir}/broker.crt" \
  "${cert_dir}/openssl-san.cnf"

openssl req \
  -new \
  -x509 \
  -days 3650 \
  -subj "/CN=event-gateway-kafka-dev-ca" \
  -passout "pass:${password}" \
  -keyout "${cert_dir}/ca.key" \
  -out "${cert_dir}/ca.crt"

keytool \
  -genkeypair \
  -alias "${broker_host}" \
  -keyalg RSA \
  -validity 3650 \
  -storetype JKS \
  -keystore "${cert_dir}/kafka.keystore.jks" \
  -storepass "${password}" \
  -keypass "${password}" \
  -dname "CN=${broker_host}, OU=Event Gateway, O=WSO2, L=Colombo, S=Western, C=LK" \
  -ext "SAN=DNS:${broker_host},DNS:localhost,IP:127.0.0.1"

keytool \
  -certreq \
  -alias "${broker_host}" \
  -keystore "${cert_dir}/kafka.keystore.jks" \
  -storepass "${password}" \
  -file "${cert_dir}/broker.csr"

cat > "${cert_dir}/openssl-san.cnf" <<EOF
[v3_req]
subjectAltName=DNS:${broker_host},DNS:localhost,IP:127.0.0.1
EOF

openssl x509 \
  -req \
  -days 3650 \
  -CA "${cert_dir}/ca.crt" \
  -CAkey "${cert_dir}/ca.key" \
  -CAcreateserial \
  -passin "pass:${password}" \
  -in "${cert_dir}/broker.csr" \
  -out "${cert_dir}/broker.crt" \
  -extfile "${cert_dir}/openssl-san.cnf" \
  -extensions v3_req

keytool \
  -importcert \
  -noprompt \
  -alias CARoot \
  -keystore "${cert_dir}/kafka.keystore.jks" \
  -storepass "${password}" \
  -file "${cert_dir}/ca.crt"

keytool \
  -importcert \
  -noprompt \
  -alias "${broker_host}" \
  -keystore "${cert_dir}/kafka.keystore.jks" \
  -storepass "${password}" \
  -file "${cert_dir}/broker.crt"

keytool \
  -importcert \
  -noprompt \
  -alias CARoot \
  -keystore "${cert_dir}/kafka.truststore.jks" \
  -storepass "${password}" \
  -file "${cert_dir}/ca.crt"

chmod 0644 "${cert_dir}/ca.crt"
chmod 0600 "${cert_dir}/kafka.keystore.jks" "${cert_dir}/kafka.truststore.jks"
