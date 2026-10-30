# Stoplight Prism with the full multi-provider LLM mock OpenAPI tree in ./mock-api.
#
# Build from repository root:
#   docker build -t llm-mock-openapi-it:local kubernetes/helm/resources/apigateway-restapi-operator-demo/llm-mock-openapi-it
#
# Load into kind (example): `kind load docker-image llm-mock-openapi-it:local`
# Then point a Deployment image at `llm-mock-openapi-it:local` instead of embedding
# `stoplight/prism` + ConfigMap openapi (see README).
FROM stoplight/prism:5.14.3
# Keep the base image's working directory: its entrypoint runs node dist/index.js relative to it.
COPY mock-api /api/
EXPOSE 4010
CMD ["mock", "/api/openapi.yaml", "--host", "0.0.0.0", "--errors", "--verboseLevel", "info"]
