# Download distribution.
wget https://github.com/wso2/api-platform/releases/download/ai-gateway/v1.1.0/wso2apip-ai-gateway-1.1.0.zip

# Unzip the downloaded distribution.
unzip wso2apip-ai-gateway-1.1.0.zip

# Start the complete stack
cd wso2apip-ai-gateway-1.1.0/


docker compose up -d

# Verify gateway controller admin endpoint is running
curl http://localhost:9094/health

# Deploy LLM provider
curl -X POST http://localhost:9090/api/management/v0.9/llm-providers \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary <llm-provider.yaml>

# Deploy the LLM proxy
curl -X POST http://localhost:9090/api/management/v0.9/llm-proxies \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary <llm-proxy.yaml>