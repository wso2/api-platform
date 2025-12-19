# Gateway Tracing

This guide explains how to implement and configure distributed tracing for the API Platform Gateway components.

## Overview

The default tracing services included in the Docker Compose configuration are **demonstration services** designed to showcase how you can observe distributed traces across gateway components in a centralized setup. These services provide a reference implementation that you can use out-of-the-box for development, testing, or as a starting point for your production tracing strategy.

**Important**: You are free to choose any tracing or observability strategy that suits your environment and requirements. The provided setup is just one of many possible configurations.

## Tracing Architecture

The default tracing stack consists of:

- **OpenTelemetry (OTLP) Collector**: Receives, processes, and exports trace data from gateway components
- **Jaeger**: Stores and visualizes distributed traces with a web UI for trace exploration and analysis

### How It Works

1. Gateway components (gateway-controller, policy-engine, router) are configured to export traces via OTLP (OpenTelemetry Protocol)
2. Components send trace spans to the OpenTelemetry Collector via gRPC (port 4317) or HTTP (port 4318)
3. The OTLP Collector processes traces (batching, adding resource attributes, etc.)
4. The OTLP Collector forwards traces to Jaeger for storage and visualization
5. Users can view and analyze traces through the Jaeger UI

### What is Distributed Tracing?

Distributed tracing tracks a request as it flows through multiple components:
- **Trace**: Represents the entire journey of a request through the system
- **Span**: Represents a single operation within a trace (e.g., policy execution, upstream call)
- **Context Propagation**: Traces are correlated across components using trace IDs and span IDs in headers

## Enabling Tracing

### Configuration Required

You need to enable tracing in the gateway configuration file and point it to your OTLP collector endpoint.

The tracing configuration is located in `gateway/configs/config.yaml`:

#### Policy Engine Tracing Configuration

```yaml
tracing:
  enabled: true                          # Set to true to enable tracing
  endpoint: otel-collector:4317          # OTLP collector gRPC endpoint
  service_version: "0.2.0"               # Service version
  batch_timeout: 1s                      # Batch timeout for exporting spans
  max_export_batch_size: 512             # Maximum spans per batch
  sampling_rate: 1.0                     # Sample rate (1.0 = 100%, 0.5 = 50%)
```

### Demonstrated Tracing Services

The tracing services included in the Docker Compose file (Jaeger and OpenTelemetry Collector) are provided as **demonstration services** to show one possible way to collect and visualize traces. You can use them as-is for development/testing, or replace them with your own tracing solution.

The gateway uses Docker Compose profiles to optionally enable these demonstration tracing services.

### Start Gateway with Demonstrated Tracing Services

To start the gateway with the demonstration tracing services enabled:

```bash
docker compose --profile tracing up -d
```

This starts:
- Core gateway services (gateway-controller, policy-engine, router) - *which export traces to OTLP collector*
- OpenTelemetry Collector - *receives and processes traces*
- Jaeger - *stores and visualizes traces*

### Start Gateway without Tracing Services

To run only the core gateway services without the demonstration tracing stack:

```bash
docker compose up -d
```

**Note:** If tracing is enabled in the configuration but the OTLP collector is not running, components will log warnings about failed trace exports. To completely disable tracing, set `enabled: false` in the configuration.

### Stop Tracing Services

To stop all services including the tracing stack:

```bash
docker compose --profile tracing down
```

**Note:** Jaeger stores traces in memory by default. Stopping the service will lose all trace data. For persistent storage, configure Jaeger with a backend database (see Jaeger documentation).

## Viewing Traces in Jaeger

Once you've started the gateway with the tracing profile, follow these steps to view distributed traces:

### Step 1: Access Jaeger UI

Open your browser and navigate to:
```
http://localhost:16686
```

### Step 2: Search for Traces

The Jaeger UI provides several ways to search for traces:

1. **Select a Service** from the dropdown:
   - `policy-engine` - View traces from the policy engine
   - `router` - View traces from the Envoy router

2. **Select an Operation** (optional):
   - Choose "all" to see all operations
   - Or select a specific operation (e.g., specific policy execution)

3. **Adjust Lookback Time Range**:
   - Default: Last 1 hour
   - Options: 5m, 15m, 1h, 6h, 12h, 1d, 2d, Custom

4. **Add Filters** (optional):
   - **Tags**: Filter by specific tag values (e.g., `http.status_code=500`)
   - **Min/Max Duration**: Filter by trace duration
   - **Limit Results**: Control number of traces returned (default: 20)

5. Click **Find Traces**

### Step 3: Analyze Trace Details

Click on any trace in the results to view detailed information:

#### Trace Timeline
- **Visual timeline** showing all spans in the trace
- **Duration bars** showing relative time spent in each operation
- **Parent-child relationships** between spans
- **Color coding** by service

#### Span Details
Click on any span to see:
- **Operation name**: What operation was performed
- **Duration**: How long it took
- **Tags**: Metadata about the operation (HTTP method, status code, etc.)
- **Logs**: Events logged during the span (errors, warnings, etc.)
- **Process**: Service name, version, and host information

#### Common Use Cases

**Finding Slow Requests:**
1. Set Min Duration filter (e.g., 1000ms)
2. Click Find Traces
3. Examine spans to identify bottlenecks

**Debugging Errors:**
1. Filter by tag: `error=true` or `http.status_code=500`
2. Click on error traces
3. Examine span logs and tags for error details

**Understanding Request Flow:**
1. Search for a specific trace ID (from logs or headers)
2. View the complete request path through all components
3. Identify which component handled which part of the request

### Step 4: Trace Comparison

You can compare multiple traces to identify patterns:
1. Select multiple traces using checkboxes
2. Click **Compare Traces** button
3. View side-by-side comparison of trace structure and timings

### Step 5: Service Dependency Graph

View how services interact:
1. Click **Dependencies** in the top navigation
2. Select time range
3. View graph showing service-to-service communication patterns

## Configuration Options

### Adjusting Sampling Rate

To reduce trace volume in high-traffic environments, adjust the sampling rate:

```yaml
tracing:
  sampling_rate: 0.1  # Sample 10% of requests
```

Sampling strategies:
- `1.0` (100%): Sample all requests - recommended for development and low-traffic environments
- `0.5` (50%): Sample half of requests - moderate traffic
- `0.1` (10%): Sample 10% of requests - high traffic
- `0.01` (1%): Sample 1% of requests - very high traffic

**Note:** Lower sampling rates reduce overhead but may miss important traces.


### Custom Service Names

Customize service names for better identification:

```yaml
policy_engine:
  service_name: policy-engine-prod-us-east-1
```

### Batch Configuration

Optimize batch settings for your environment:

```yaml
tracing:
  batch_timeout: 5s              # Wait up to 5s before exporting
  max_export_batch_size: 1024    # Export up to 1024 spans per batch
```

**Lower timeout**: Faster trace visibility, more network overhead
**Higher timeout**: Better batching efficiency, slower trace visibility

## Alternative Tracing Backends

While the default setup uses Jaeger, the gateway components use OpenTelemetry and can export to any OTLP-compatible backend.

### Moesif

Moesif provides API analytics and monitoring with support for OpenTelemetry traces. It treats each HTTP request/response span as an API event for detailed analytics.

**No additional Docker services required** - Moesif is a cloud-based SaaS platform. You only need to configure the OTLP Collector to export traces to Moesif's API.

#### Configuration

Update the OTLP Collector configuration (`gateway/observability/otel-collector/config.yaml`) to export to Moesif:

```yaml
exporters:
  # Export to Moesif
  otlphttp:
    endpoint: https://api.moesif.net/v1/traces
    headers:
      X-Moesif-Application-Id: 'your-moesif-application-id'

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch, resource]
      exporters: [otlphttp]  # Send to Moesif
```

**Important Notes:**
- The endpoint uses HTTPS (not HTTP)
- Use the `otlphttp` exporter (not `otlp` which uses gRPC)
- The `X-Moesif-Application-Id` header is required for authentication

#### Obtaining Your Moesif Application ID

1. Sign up for a Moesif account at [moesif.com](https://www.moesif.com)
2. Log in to your Moesif dashboard
3. Navigate to **Settings** → **Installation** or **API Keys**
4. Locate the **Collector Application ID** field
5. Copy your unique Application ID

#### Using Environment Variables

For better security, use environment variables for the Application ID:

```yaml
exporters:
  otlphttp:
    endpoint: https://api.moesif.net/v1/traces
    headers:
      X-Moesif-Application-Id: '${MOESIF_APPLICATION_ID}'
```

Update `docker-compose.yaml` to pass the environment variable:

```yaml
otel-collector:
  image: otel/opentelemetry-collector:latest
  environment:
    - MOESIF_APPLICATION_ID=${MOESIF_APPLICATION_ID}
  # ... rest of configuration
```

Set the environment variable before starting:

```bash
export MOESIF_APPLICATION_ID=your-moesif-application-id
docker compose --profile tracing up -d
```

#### Accessing Moesif Dashboard

After configuring and starting the gateway:

1. Navigate to [moesif.com](https://www.moesif.com) and log in
2. Go to **Events** → **Live Event Log** to see incoming API events
3. View API analytics, user behavior, and performance metrics
4. Use **Time Series** to analyze API usage trends
5. Set up **Alerts** for error rates, latency, or custom conditions

#### Moesif Features

- **API Analytics**: Request volume, response times, error rates
- **User Tracking**: Identify and track API users across requests
- **Error Analysis**: Detailed error tracking with request/response bodies
- **Behavioral Cohorts**: Group users by API usage patterns
- **Custom Dashboards**: Build visualizations for your specific KPIs
- **Alerting**: Get notified of anomalies or threshold breaches

#### Sending to Both Jaeger and Moesif

You can send traces to both Jaeger (for development) and Moesif (for analytics):

```yaml
exporters:
  # Local Jaeger for development
  otlp/jaeger:
    endpoint: jaeger:4317
    tls:
      insecure: true

  # Moesif for analytics
  otlphttp/moesif:
    endpoint: https://api.moesif.net/v1/traces
    headers:
      X-Moesif-Application-Id: '${MOESIF_APPLICATION_ID}'

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch, resource]
      exporters: [otlp/jaeger, otlphttp/moesif]
```

### Zipkin

Replace Jaeger with Zipkin:

```yaml
zipkin:
  image: openzipkin/zipkin:latest
  ports:
    - "9411:9411"
  networks:
    - gateway-network
```

Update OTLP Collector configuration to export to Zipkin:

```yaml
exporters:
  zipkin:
    endpoint: http://zipkin:9411/api/v2/spans
```

Access Zipkin UI at `http://localhost:9411`

### Grafana Tempo

For a Prometheus-style tracing backend:

```yaml
tempo:
  image: grafana/tempo:latest
  command: ["-config.file=/etc/tempo.yaml"]
  volumes:
    - ./observability/tempo/tempo.yaml:/etc/tempo.yaml
    - tempo-data:/tmp/tempo
  ports:
    - "3200:3200"   # Tempo HTTP
    - "4317:4317"   # OTLP gRPC
  networks:
    - gateway-network

grafana:
  image: grafana/grafana:latest
  environment:
    - GF_AUTH_ANONYMOUS_ENABLED=true
    - GF_AUTH_ANONYMOUS_ORG_ROLE=Admin
  ports:
    - "3000:3000"
  volumes:
    - ./observability/grafana/datasources.yaml:/etc/grafana/provisioning/datasources/datasources.yaml
  networks:
    - gateway-network
  depends_on:
    - tempo
```

Configure gateway to send directly to Tempo:
```yaml
tracing:
  endpoint: tempo:4317
```

### Cloud-Native Tracing Solutions

#### AWS X-Ray

Configure OTLP Collector to export to AWS X-Ray:

```yaml
exporters:
  awsxray:
    region: us-east-1
    no_verify_ssl: false
```

Or use the AWS Distro for OpenTelemetry (ADOT) Collector:

```yaml
otel-collector:
  image: public.ecr.aws/aws-observability/aws-otel-collector:latest
  command: ["--config=/etc/otel-collector-config.yaml"]
  environment:
    - AWS_REGION=us-east-1
```

#### Google Cloud Trace

Configure OTLP Collector to export to Google Cloud:

```yaml
exporters:
  googlecloud:
    project: your-gcp-project-id
    use_insecure: false
```

#### Azure Monitor

Use Azure Monitor exporter:

```yaml
exporters:
  azuremonitor:
    instrumentation_key: "your-instrumentation-key"
```

#### Datadog APM

Configure OTLP Collector to export to Datadog:

```yaml
exporters:
  datadog:
    api:
      key: ${DD_API_KEY}
      site: datadoghq.com
```

Or use Datadog Agent directly:

```yaml
datadog-agent:
  image: datadog/agent:latest
  environment:
    - DD_API_KEY=${DD_API_KEY}
    - DD_APM_ENABLED=true
    - DD_APM_NON_LOCAL_TRAFFIC=true
    - DD_OTLP_CONFIG_RECEIVER_PROTOCOLS_GRPC_ENDPOINT=0.0.0.0:4317
  ports:
    - "4317:4317"
  networks:
    - gateway-network
```

Update gateway configuration:
```yaml
tracing:
  endpoint: datadog-agent:4317
```

#### New Relic

Configure OTLP Collector to export to New Relic:

```yaml
exporters:
  otlphttp:
    endpoint: https://otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
```

#### Honeycomb

```yaml
exporters:
  otlp:
    endpoint: api.honeycomb.io:443
    headers:
      x-honeycomb-team: ${HONEYCOMB_API_KEY}
```

#### Lightstep

```yaml
exporters:
  otlp:
    endpoint: ingest.lightstep.com:443
    headers:
      lightstep-access-token: ${LIGHTSTEP_ACCESS_TOKEN}
```

### Service Mesh Integration

If using a service mesh like Istio or Linkerd:

#### Istio

Istio automatically generates traces for service-to-service communication. Configure gateway components to propagate trace context:

```yaml
tracing:
  enabled: true
  endpoint: istio-telemetry.istio-system:4317
```

#### Linkerd

Linkerd integrates with Jaeger via OpenTelemetry:

```yaml
tracing:
  enabled: true
  endpoint: linkerd-collector.linkerd:4317
```

## Customizing OpenTelemetry Collector

The OTLP Collector configuration is located at:
```
gateway/observability/otel-collector/config.yaml
```

### Configuration Structure

The configuration consists of three main sections:

#### Receivers
Define how traces are received:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
```

#### Processors
Transform and enrich traces:

```yaml
processors:
  # Batch spans for efficiency
  batch:
    timeout: 1s
    send_batch_size: 1024

  # Add resource attributes
  resource:
    attributes:
      - key: environment
        value: production
        action: upsert
      - key: cluster
        value: us-west-2
        action: upsert

  # Memory limiter to prevent OOM
  memory_limiter:
    check_interval: 1s
    limit_mib: 512

  # Sampling processor
  probabilistic_sampler:
    sampling_percentage: 10  # Sample 10% of traces
```

#### Exporters
Define where traces are sent:

```yaml
exporters:
  # Send to Jaeger
  otlp:
    endpoint: jaeger:4317
    tls:
      insecure: true

  # Debug output to console
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 200
```

#### Service Pipeline
Connect receivers, processors, and exporters:

```yaml
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch, resource]
      exporters: [otlp, debug]
```

### Example: Multi-Backend Export

Send traces to multiple backends simultaneously:

```yaml
exporters:
  otlp/jaeger:
    endpoint: jaeger:4317
    tls:
      insecure: true

  otlp/tempo:
    endpoint: tempo:4317
    tls:
      insecure: true

  datadog:
    api:
      key: ${DD_API_KEY}

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp/jaeger, otlp/tempo, datadog]
```

### Example: Tail-Based Sampling

Keep all error traces but sample successful traces:

```yaml
processors:
  tail_sampling:
    policies:
      - name: error-traces
        type: status_code
        status_code:
          status_codes: [ERROR]
      - name: slow-traces
        type: latency
        latency:
          threshold_ms: 1000
      - name: probabilistic
        type: probabilistic
        probabilistic:
          sampling_percentage: 10
```

## Trace Context Propagation

The gateway components automatically propagate trace context using standard W3C Trace Context headers:

- `traceparent`: Contains trace ID, span ID, and sampling decision
- `tracestate`: Contains vendor-specific trace information

When making requests to the gateway, you can:

1. **Let the gateway create a new trace** (default)
2. **Propagate your own trace context** by including trace headers:

```bash
curl http://localhost:8080/weather/v1.0/us/seattle \
  -H "traceparent: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
```

This allows you to trace requests across your entire system, including services before and after the gateway.

## Best Practices

### Development
- Use 100% sampling rate (`sampling_rate: 1.0`)
- Enable debug output in OTLP collector
- Use Jaeger for quick trace visualization
- Keep trace data for 1-7 days

### Production
- **Use managed services** (Datadog, New Relic, etc.) to reduce operational overhead
- **Implement appropriate sampling** (1-10% depending on traffic volume)
- **Enable TLS** for OTLP connections
- **Set resource limits** on OTLP collector
- **Monitor collector health** and performance
- **Implement trace retention policies** based on compliance and storage costs
- **Use tail-based sampling** to keep important traces (errors, slow requests)

### Security
- Enable TLS for trace transmission
- Sanitize sensitive data from trace attributes
- Implement proper access controls for trace viewing
- Regularly audit who accesses trace data
- Consider data residency requirements

### Performance
- Use appropriate sampling rates to balance visibility and overhead
- Configure batch settings to optimize network usage
- Monitor gateway component overhead from tracing
- Use asynchronous trace export (default with OTLP)
- Consider using tail-based sampling for high-volume environments

### Sampling Strategy

Choose sampling based on traffic volume:

| Traffic Volume | Sampling Rate | Use Case |
|---------------|---------------|----------|
| < 100 req/s | 100% (1.0) | Full visibility, low overhead |
| 100-1000 req/s | 10-50% (0.1-0.5) | Balanced visibility and cost |
| 1000-10000 req/s | 1-10% (0.01-0.1) | Cost-effective, statistical sampling |
| > 10000 req/s | 0.1-1% (0.001-0.01) | Minimal overhead, error sampling |

**Note:** Always use 100% sampling for errors using tail-based sampling.

## Troubleshooting

### Traces Not Appearing in Jaeger

**1. Verify tracing is enabled in configuration:**
```bash
cat gateway/configs/config.yaml | grep -A5 "tracing:"
```

Ensure `enabled: true`.

**2. Check OTLP Collector is running:**
```bash
docker ps | grep otel-collector
```

**3. View OTLP Collector logs:**
```bash
docker logs otel-collector
```

Look for connection errors or export failures.

**4. Check Jaeger is running:**
```bash
docker ps | grep jaeger
curl http://localhost:16686/
```

**5. Verify network connectivity:**
```bash
docker exec policy-engine ping otel-collector
docker exec otel-collector ping jaeger
```

**6. Check gateway component logs for trace export errors:**
```bash
docker logs policy-engine | grep -i trace
docker logs gateway-controller | grep -i trace
```

### Traces Are Incomplete or Missing Spans

**1. Check sampling rate** - ensure it's not too low
**2. Verify all components are configured** to export traces
**3. Check for trace context propagation issues** - ensure headers are preserved
**4. Look for timeout errors** in OTLP collector logs

### High Trace Export Overhead

**1. Reduce sampling rate:**
```yaml
tracing:
  sampling_rate: 0.1  # Reduce from 1.0 to 0.1
```

**2. Increase batch size:**
```yaml
tracing:
  batch_timeout: 5s
  max_export_batch_size: 2048
```

**3. Use tail-based sampling** in OTLP collector to sample only important traces

### Traces Have Incorrect Timing

- **Ensure system clocks are synchronized** across all containers (use NTP)
- **Check for clock skew** in trace timeline view
- **Verify trace context propagation** is working correctly

### Cannot Access Jaeger UI

**1. Verify Jaeger is running:**
```bash
docker ps | grep jaeger
```

**2. Check Jaeger logs:**
```bash
docker logs jaeger
```

**3. Ensure port 16686 is not blocked:**
```bash
curl http://localhost:16686/
```

## Disabling Tracing

To completely disable tracing:

1. **Update configuration** in `gateway/configs/config.yaml`:

```yaml
policy_engine:
  tracing:
    enabled: false

tracing:
  enabled: false
```

2. **Restart gateway services:**

```bash
docker compose restart gateway-controller policy-engine router
```

**Note:** The router (Envoy) tracing is controlled by the gateway-controller configuration and will be disabled when the configuration is updated.

## Integration with Logging

Traces and logs work together for comprehensive observability:

### Correlating Traces and Logs

1. **Trace ID in Logs**: Gateway components include trace IDs in log entries
2. **Find Trace from Log**: Copy trace ID from log entry and search in Jaeger
3. **Find Logs from Trace**: Copy trace ID from Jaeger and search in log viewer

Example log entry with trace ID:
```json
{
  "level": "info",
  "ts": "2025-12-19T10:30:45.456Z",
  "msg": "Policy executed",
  "trace_id": "0af7651916cd43dd8448eb211c80319c",
  "span_id": "b7ad6b7169203331",
  "policy": "ModifyHeaders"
}
```

### Using Both Stacks

Enable both logging and tracing profiles:

```bash
docker compose --profile logging --profile tracing up -d
```

This provides complete observability:
- **Traces**: Request flow and performance
- **Logs**: Detailed event information and debugging

## Additional Resources

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
- [OpenTelemetry Collector Documentation](https://opentelemetry.io/docs/collector/)
- [W3C Trace Context Specification](https://www.w3.org/TR/trace-context/)
- [Gateway README](README.md)
- [Gateway Logging Guide](logging.md)
- [Docker Compose Profiles](https://docs.docker.com/compose/profiles/)
