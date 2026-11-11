# Safeguard LLM traffic with Amazon Bedrock Guardrails and WSO2 AI Gateway

Code-only version of the sample for the AWS Machine Learning Blog post *Safeguard LLM traffic with Amazon Bedrock Guardrails and WSO2 AI Gateway*. It runs a standalone gateway and configures it through the gateway's management API.

- **AWS side, as code:** Terraform creates two Amazon Bedrock guardrails (an organization baseline and a support-assistant guardrail), their versions, and a least-privilege IAM role for the gateway.
- **WSO2 side, as code:** `deploy-artifacts.sh` deploys the LLM provider, the two App LLM proxies, and their guardrail policies to a gateway that runs on your machine.
- **No stored AWS keys:** your AWS CLI session's temporary credentials only call `sts:AssumeRole` on the gateway role. Every Amazon Bedrock call runs under that role.

![High-level architecture](images/figure-1-architecture.png)

## Layout

| Path | Contents |
| --- | --- |
| `terraform/` | The guardrails, their versions, the gateway's IAM role, and an Amazon CloudWatch dashboard |
| `compose/` | `docker-compose.override.yaml`, which passes your session credentials to the gateway runtime, and the guardrail allowlist |
| `scripts/` | Start the gateway, deploy the provider and proxies, test, and clean up |
| `gateway/` | Provider and proxy definitions |

## Prerequisites

- An AWS account and an AWS CLI session (AWS IAM Identity Center recommended).
- An AWS Region with Amazon Bedrock and Amazon Bedrock Guardrails. The post uses `us-east-1` with the `us.amazon.nova-2-lite-v1:0` inference profile, which routes to `us-east-1`, `us-east-2`, and `us-west-2`. Set `model_regions` if you use another profile.
- Docker with Docker Compose 2.30 or later, Terraform 1.9 or later, AWS CLI 2.9 or later, curl, jq, envsubst, and unzip.

## Run

```bash
aws sso login
export AWS_REGION=us-east-1 BEDROCK_MODEL_ID=us.amazon.nova-2-lite-v1:0
TF_VARS=(-var="region=${AWS_REGION}" -var="model_id=${BEDROCK_MODEL_ID}")

terraform -chdir=terraform init
terraform -chdir=terraform apply "${TF_VARS[@]}"

./scripts/start-gateway.sh        # downloads the gateway (SHA-256 verified), sets it up, and starts it
./scripts/deploy-artifacts.sh     # provider, both proxies, and one key per app in Secrets Manager
./scripts/test-guardrails.sh wso2apip-ai-gateway   # calls https://localhost:8443/<proxy>
terraform -chdir=terraform output -raw dashboard_url   # CloudWatch dashboard with checks per guardrail
```

Terraform also creates an Amazon CloudWatch dashboard that graphs each guardrail's checks and the requests and responses that the guardrails block or mask. Metrics appear a few minutes after traffic. Add `-var="create_dashboard=false"` to skip it; `terraform destroy` removes it.

`start-gateway.sh` runs `run-gateway.sh`, which starts the gateway in the background, confirms that `gateway-runtime` received your AWS credentials (it prints variable names only), and returns. Set `FOLLOW_LOGS=1` to follow the logs. Stop the gateway with `docker compose --env-file api-platform.env down` in the gateway folder.

`deploy-artifacts.sh` deploys `gateway/bedrock-provider.yaml` and the two proxy files, fills in the Terraform outputs, and stores each application's key in Secrets Manager without printing it. On its first run, `start-gateway.sh` asks you to choose an admin password for the management API (or reads `ADMIN_PASSWORD`) and keeps it in Secrets Manager. The proxies live at `https://localhost:8443/<proxy>`. Rerun `run-gateway.sh wso2apip-ai-gateway` to restart the gateway with fresh credentials when your AWS session expires.

## Gateway version and the guardrail policy

The sample uses WSO2 AI Gateway 1.2.0, which ships `aws-bedrock-guardrail` v1.0.2. On first start, `run-gateway.sh` runs `build-gateway-image.sh`, which pins the policy to v1.3.0 in the gateway's `build.yaml` and builds new controller and runtime images with the [`ap` CLI](https://github.com/wso2/api-platform/releases/tag/ap%2Fv0.9.1). The script downloads `ap` v0.9.1 into `.tools/` in this folder, verifies its SHA-256 checksum, and doesn't add it to your `PATH`; `cleanup.sh` removes it. `ap` keeps its own settings in `~/.wso2ap/`. v1.3.0 adds `applicableStatusCodes`, so a response check skips error responses, such as a 422 from another guardrail. The generated `docker-compose.override.yaml` points the gateway at the built images.

## Clean up

```bash
./scripts/cleanup.sh wso2apip-ai-gateway
terraform -chdir=terraform destroy "${TF_VARS[@]}"
```

## Scan results

| Tool | Scope | Result |
| --- | --- | --- |
| Checkov 3 | `terraform/` | 8 passed, 0 failed |
| ShellCheck | `scripts/*.sh` | No findings |
| yamllint | `compose/` | No findings |

## License

Apache License 2.0. See [LICENSE](LICENSE).
