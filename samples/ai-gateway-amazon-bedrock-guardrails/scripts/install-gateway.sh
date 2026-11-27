#!/usr/bin/env bash
# Installs cert-manager and the gateway's Helm chart in the EKS cluster, with private services. Run it
# after ./scripts/prepare-gateway.sh. Rerun it to apply changes.
# Usage: ./scripts/install-gateway.sh
set -euo pipefail
# shellcheck source=scripts/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

CHART="oci://ghcr.io/wso2/api-platform/helm-charts/gateway"
CHART_VERSION="1.2.5"
CERT_MANAGER_VERSION="v1.19.1"

[ -f "${GATEWAY_DIR}/.sample-images" ] || { echo "No built images. Run ./scripts/prepare-gateway.sh first." >&2; exit 1; }
# shellcheck disable=SC1091 # Written by prepare-gateway.sh.
source "${GATEWAY_DIR}/.sample-images"

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT
chmod 700 "${WORK_DIR}"

aws eks update-kubeconfig --name "${CLUSTER_NAME}" --region "${AWS_REGION}" >/dev/null

# 1. cert-manager, which the chart uses for its TLS certificates.
helm upgrade --install cert-manager oci://quay.io/jetstack/charts/cert-manager \
  --version "${CERT_MANAGER_VERSION}" --namespace cert-manager --create-namespace \
  --set crds.enabled=true --wait

kubectl get namespace "${NAMESPACE}" >/dev/null 2>&1 || kubectl create namespace "${NAMESPACE}"

# 2. The admin password, generated once and kept in Secrets Manager. The chart receives only a bcrypt hash.
if ! aws secretsmanager describe-secret --secret-id "${ADMIN_SECRET}" >/dev/null 2>&1; then
  openssl rand -base64 24 | tr -d '\n' | aws secretsmanager create-secret --name "${ADMIN_SECRET}" \
    --description "WSO2 AI Gateway management API admin password" \
    --secret-string file:///dev/stdin >/dev/null
  echo "Stored a generated admin password in Secrets Manager as ${ADMIN_SECRET}."
fi
# shellcheck disable=SC2016 # The dollar signs in the sed expression are literal.
ADMIN_HASH="$(aws secretsmanager get-secret-value --secret-id "${ADMIN_SECRET}" --query SecretString --output text \
  | docker run --rm -i httpd:2.4-alpine htpasswd -niBC 10 admin 2>/dev/null | cut -d: -f2 | tr -d '\r\n' | sed 's/^\$2y\$/$2a$/')"
[ -n "${ADMIN_HASH}" ] || { echo "Couldn't hash the admin password." >&2; exit 1; }

# 3. The key that encrypts the gateway's secrets at rest, generated once.
if ! kubectl -n "${NAMESPACE}" get secret gateway-encryption-keys >/dev/null 2>&1; then
  openssl rand 32 > "${WORK_DIR}/default-aesgcm256-v1.bin"
  kubectl -n "${NAMESPACE}" create secret generic gateway-encryption-keys \
    --from-file=default-aesgcm256-v1.bin="${WORK_DIR}/default-aesgcm256-v1.bin"
fi

# 4. The database password, copied from the secret that Amazon RDS manages.
aws secretsmanager get-secret-value --secret-id "$(tf_output db_secret_arn)" --query SecretString --output text \
  | jq -jr .password > "${WORK_DIR}/password"
kubectl -n "${NAMESPACE}" create secret generic gateway-db --from-file=password="${WORK_DIR}/password" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null

# 5. The settings that come from this deployment.
cat > "${WORK_DIR}/values.yaml" <<VALUES
gateway:
  config:
    controller:
      auth:
        basic:
          enabled: true
          users:
            - username: admin
              password: '${ADMIN_HASH}'
              password_hashed: true
              roles: ["admin"]
      storage:
        postgres:
          host: $(tf_output db_host)
          database: $(tf_output db_name)
          user: $(tf_output db_user)
  controller:
    image:
      repository: ${CONTROLLER_IMAGE%:*}
      tag: "${CONTROLLER_IMAGE##*:}"
  gatewayRuntime:
    image:
      repository: ${RUNTIME_IMAGE%:*}
      tag: "${RUNTIME_IMAGE##*:}"
VALUES

# 6. Create the database tables. The controller doesn't create them on PostgreSQL. The script only adds
#    missing tables, so it's safe to rerun.
kubectl -n "${NAMESPACE}" create configmap gateway-db-schema \
  --from-file=schema.sql="${GATEWAY_DIR}/resources/gateway-controller/db-scripts/gateway-controller-db.postgres.sql" \
  --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl -n "${NAMESPACE}" delete job gateway-db-schema --ignore-not-found --wait=true >/dev/null
kubectl -n "${NAMESPACE}" apply -f - >/dev/null <<JOB
apiVersion: batch/v1
kind: Job
metadata:
  name: gateway-db-schema
spec:
  backoffLimit: 2
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: psql
          image: public.ecr.aws/docker/library/postgres:17-alpine
          command: ["psql", "-v", "ON_ERROR_STOP=1", "-f", "/schema/schema.sql"]
          env:
            - { name: PGHOST, value: "$(tf_output db_host)" }
            - { name: PGDATABASE, value: "$(tf_output db_name)" }
            - { name: PGUSER, value: "$(tf_output db_user)" }
            - { name: PGSSLMODE, value: require }
            - name: PGPASSWORD
              valueFrom:
                secretKeyRef: { name: gateway-db, key: password }
          volumeMounts:
            - { name: schema, mountPath: /schema }
      volumes:
        - name: schema
          configMap: { name: gateway-db-schema }
JOB
kubectl -n "${NAMESPACE}" wait --for=condition=complete job/gateway-db-schema --timeout=5m

# 7. Unpack the chart, add the guardrail allowlist to its config.toml, and install it.
helm pull "${CHART}" --version "${CHART_VERSION}" --untar --untardir "${WORK_DIR}"
"${ROOT_DIR}/helm/add-allowlist.sh" "${WORK_DIR}/gateway"
helm upgrade --install "${RELEASE}" "${WORK_DIR}/gateway" --namespace "${NAMESPACE}" \
  -f "${ROOT_DIR}/helm/values.yaml" -f "${WORK_DIR}/values.yaml" --wait --timeout 10m

kubectl -n "${NAMESPACE}" rollout status "deployment/${RELEASE}-controller"
kubectl -n "${NAMESPACE}" rollout status "deployment/${RELEASE}-gateway-runtime"

# 8. Save the gateway's listener certificate, so curl can verify it through the port forward. The chart's
#    cert-manager Issuer signs it for localhost.
kubectl -n "${NAMESPACE}" get secret "${RELEASE}-controller-tls" -o jsonpath='{.data.tls\.crt}' \
  | base64 --decode > "${GATEWAY_DIR}/listener.crt"
echo "Saved the gateway's certificate to ${GATEWAY_DIR#"${ROOT_DIR}"/}/listener.crt."
echo "The gateway is running. Next, in a second terminal: ./scripts/port-forward.sh"
