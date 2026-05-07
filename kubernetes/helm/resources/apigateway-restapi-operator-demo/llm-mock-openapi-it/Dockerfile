# Stoplight Prism with the full gateway integration-test OpenAPI tree
# (`gateway/it/mock-api`), matching `mock-openapi` in `gateway/it/docker-compose.test.yaml`.
#
# Build from repository root:
#   docker build -f kubernetes/helm/resources/apigateway-restapi-operator-demo/llm-mock-openapi-it/Dockerfile -t llm-mock-openapi-it:local gateway/it
#
# Load into kind (example): `kind load docker-image llm-mock-openapi-it:local`
# Then point a Deployment image at `llm-mock-openapi-it:local` instead of embedding
# `stoplight/prism` + ConfigMap openapi (see README).
FROM stoplight/prism:5.14.3
WORKDIR /api
COPY mock-api /api/
EXPOSE 4010
CMD ["mock", "/api/openapi.yaml", "--host", "0.0.0.0", "--errors", "--verboseLevel", "info"]
