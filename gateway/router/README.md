# Router (Envoy Proxy)

The Router is an Envoy Proxy-based data plane that routes HTTP traffic to backend services based on configurations received from the Gateway-Controller via xDS.

## Features

- **Envoy Proxy 1.35.3**: Industry-standard, high-performance proxy
- **Dynamic Configuration**: xDS protocol for zero-downtime updates
- **Graceful Reload**: In-flight requests complete during configuration changes
- **Structured Access Logs**: JSON-formatted logs to stdout for observability
- **Health Monitoring**: Admin interface on port 9901
- **Resilient Startup**: Waits for Gateway-Controller with exponential backoff

## Architecture

```
HTTP Traffic (Port 8080)
      ↓
  Envoy Proxy
      ↑ (xDS updates)
Gateway-Controller (Port 18000)
```

## Building

### Build Docker Image

```bash
make docker
```

## Running

### Docker

```bash
docker run -p 8080:8080 -p 9901:9901 \
  -e XDS_SERVER_HOST=gateway-controller:18000 \
  wso2/gateway-router:latest
```

### Docker Compose

The easiest way to run the complete gateway system:

```bash
cd ../
docker compose up -d
```

## Configuration

### Environment Variables

- `XDS_SERVER_HOST`: Gateway-Controller xDS endpoint - default: `gateway-controller:18000`

### Envoy Bootstrap

The Router uses a minimal bootstrap configuration (`config/envoy-bootstrap.yaml`) that:

1. Configures admin interface on port 9901
2. Sets up xDS cluster pointing to Gateway-Controller
3. Configures dynamic resource discovery (LDS, CDS, RDS)
4. Implements retry policy with exponential backoff

## Startup Behavior

When the Router starts:

1. Connects to Gateway-Controller xDS server (port 18000)
2. If connection fails, retries with exponential backoff:
   - Base interval: 1 second
   - Max interval: 30 seconds
   - Retries indefinitely until successful
3. Once connected, receives initial xDS snapshot
4. Begins routing traffic based on configuration

**Important**: Router will NOT serve traffic until it receives valid configuration from Gateway-Controller.

## Monitoring

### Admin Interface

Access Envoy's admin interface at `http://localhost:9901`:

- `/stats` - Proxy statistics
- `/config_dump` - Current configuration
- `/clusters` - Upstream cluster status
- `/listeners` - Active listeners
- `/ready` - Readiness check

### Health Checks

```bash
# Check if Router is ready
curl http://localhost:9901/ready

# View configuration
curl http://localhost:9901/config_dump

# View statistics
curl http://localhost:9901/stats
```

### Access Logs

The Router emits structured JSON access logs to stdout for all HTTP requests, providing observability for production environments.

#### Log Format

All requests are logged in JSON format with 16 standard fields:

```json
{
  "start_time": "2025-10-12T10:30:45.123Z",
  "method": "GET",
  "path": "/weather/US/Seattle",
  "protocol": "HTTP/1.1",
  "response_code": 200,
  "response_flags": "-",
  "bytes_received": 0,
  "bytes_sent": 1024,
  "duration": 45,
  "upstream_service_time": "42",
  "x_forwarded_for": "192.168.1.10",
  "user_agent": "curl/7.68.0",
  "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "authority": "gateway.example.com",
  "upstream_host": "api.weather.com:443",
  "upstream_cluster": "cluster_api_weather_com"
}
```

#### Viewing Logs

**Docker**:
```bash
docker logs -f router
```

**Docker Compose**:
```bash
docker compose logs -f router
```

**Kubernetes**:
```bash
kubectl logs -f deployment/gateway-router
```

#### Log Fields Reference

| Field | Description |
|-------|-------------|
| `start_time` | Request timestamp (ISO 8601) |
| `method` | HTTP method (GET, POST, etc.) |
| `path` | Request path |
| `protocol` | HTTP protocol version |
| `response_code` | HTTP status code |
| `response_flags` | Envoy response flags (`-` = success, `UH` = no healthy upstream, `UF` = upstream failure, `NR` = no route) |
| `bytes_received` | Request body size |
| `bytes_sent` | Response body size |
| `duration` | Total request duration (ms) |
| `upstream_service_time` | Backend processing time (ms) |
| `x_forwarded_for` | Client IP address |
| `user_agent` | Client user agent |
| `request_id` | Unique request ID (UUID) |
| `authority` | HTTP Host header |
| `upstream_host` | Backend server address |
| `upstream_cluster` | Backend cluster name |

#### Log Aggregation

The JSON format is compatible with common log aggregation platforms:

- **ELK Stack**: Logstash natively parses JSON
- **Splunk**: JSON log format supported
- **CloudWatch Logs**: AWS CloudWatch Logs Insights
- **Datadog**: Automatic JSON parsing
- **Loki/Grafana**: Query with `| json` parser

**Example Loki query** (find slow requests):
```logql
{container="gateway-router"} | json | duration > 1000
```

**Example Splunk query** (find 5xx errors):
```spl
sourcetype="gateway-router" response_code>=500
```

#### Configuration

Access logs are configured dynamically by the Gateway-Controller in the xDS Listener resources. No Router-side configuration is needed.

## Traffic Routing

Once configured, the Router routes traffic based on API configurations:

### Example

With a Weather API configured:
- Context: `/weather`
- Upstream: `http://api.weather.com/api/v2`
- Operations: `GET /{country}/{city}`

Requests to:
```bash
curl http://localhost:8080/weather/US/Seattle
```

Are routed to:
```
http://api.weather.com/api/v2/US/Seattle
```

## Configuration Updates

The Router receives configuration updates from Gateway-Controller automatically:

1. User submits API configuration to Gateway-Controller
2. Gateway-Controller validates and generates xDS snapshot
3. Gateway-Controller pushes snapshot to Router via xDS
4. Router applies configuration gracefully:
   - Existing connections complete with old config
   - New connections use new config
   - Zero dropped requests

## Troubleshooting

### Router not starting

Check logs:
```bash
docker logs router
```

Verify Gateway-Controller is running:
```bash
curl http://localhost:9090/health
```

### Router not routing traffic

1. Check admin interface: `curl http://localhost:9901/config_dump`
2. Verify API configuration exists: `curl http://localhost:9090/apis`
3. Check Router logs for errors: `docker logs router`

### Connection refused errors

Ensure:
1. Gateway-Controller is running and accessible on port 18000
2. XDS_SERVER_HOST environment variable points to correct address
3. No firewall blocking port 18000

## Performance

- Routes millions of requests per second
- Sub-millisecond latency overhead
- Handles 100+ configured APIs
- Zero-downtime configuration updates

## Envoy Version

This Router uses Envoy Proxy v1.35.3, which includes:
- Full xDS v3 API support
- HTTP/1.1, HTTP/2, and HTTP/3 support
- Advanced load balancing
- Circuit breaking
- Health checking
- Connection pooling

## License

Based on Envoy Proxy (Apache 2.0 License)
Copyright WSO2. All rights reserved.
