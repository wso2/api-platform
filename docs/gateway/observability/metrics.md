# Gateway Metrics

This guide explains how to implement and configure metrics collection for the API Platform Gateway components.

## Overview

The default metrics services included in the Docker Compose configuration are **demonstration services** designed to showcase how you can observe component metrics in a centralized setup. These services provide a reference implementation that you can use out-of-the-box for development, testing, or as a starting point for your production metrics strategy.

**Important**: You are free to choose any metrics or observability strategy that suits your environment and requirements. The provided setup is just one of many possible configurations.

## Metrics Architecture

The default metrics stack consists of:

- **Prometheus**: Scrapes, stores, and queries metrics from gateway components
- **Grafana**: Visualizes metrics through dashboards with alerts and notifications

### How It Works

1. Gateway components (gateway-controller, policy-engine, router) expose metrics via Prometheus HTTP endpoints
2. Prometheus scrapes these endpoints at regular intervals (default: 15s)
3. Metrics are stored in Prometheus's time-series database
4. Grafana queries Prometheus to visualize metrics through pre-built dashboards
5. Users can view real-time metrics, historical trends, and set up alerts

### What are Metrics?

Metrics are numerical measurements tracked over time:

- **Counters**: Cumulative values that only increase (e.g., total requests, total errors)
- **Gauges**: Current values that can go up or down (e.g., active connections, memory usage)
- **Histograms**: Sample observations with configurable buckets (e.g., request duration)

## Enabling Metrics

### Configuration Required

You need to enable metrics in the gateway configuration file. By default, metrics are enabled in the production configuration but you can customize the settings.

The metrics configuration is located in `gateway/configs/config.yaml`:

#### Gateway Controller Metrics Configuration

```yaml
gateway_controller:
  metrics:
    # Enable or disable Prometheus metrics endpoint
    enabled: true

    # Port for metrics HTTP server
    port: 9091
```

#### Policy Engine Metrics Configuration

```yaml
policy_engine:
  metrics:
    # Enable or disable Prometheus metrics endpoint
    enabled: true

    # Port for metrics HTTP server
    port: 9003
```

**Note**: When metrics are enabled, each component starts an HTTP server on the specified port to expose metrics in Prometheus format.

### Demonstrated Metrics Services

The metrics services included in the Docker Compose file (Prometheus and Grafana) are provided as **demonstration services** to show one possible way to collect and visualize metrics. You can use them as-is for development/testing, or replace them with your own metrics solution.

The gateway uses Docker Compose profiles to optionally enable these demonstration metrics services.

#### Setting up Grafana Image

**Important Note:** The Grafana image in the `docker-compose.yaml` file is intentionally left empty due to licensing considerations. Before you can use the demonstration dashboards, you must specify a valid Grafana image.

To add the Grafana image:

1. Open `gateway/docker-compose.yaml`.
2. Locate the `grafana` service definition.
3. Update the `image` field with a valid Grafana image tag (e.g., `grafana/grafana:11.6.0` or another compatible version).

```yaml
  grafana:
    image: grafana/grafana:11.6.0  # Add your preferred Grafana image here
    container_name: grafana
```

### Start Gateway with Demonstrated Metrics Services

To start the gateway with the demonstration metrics services enabled:

```bash
docker compose --profile metrics up -d
```

This starts:
- Core gateway services (gateway-controller, policy-engine, router) - *which expose metrics on their respective ports*
- Prometheus - *scrapes and stores metrics*
- Grafana - *visualizes metrics through dashboards*

### Start Gateway without Metrics Services

To run only the core gateway services without the demonstration metrics stack:

```bash
docker compose up -d
```

**Note:** The gateway components still expose metrics if enabled in the configuration. You can still access metrics directly at:
- Gateway Controller: <http://localhost:9091/metrics>
- Policy Engine: <http://localhost:9003/metrics>
- Router (Envoy): <http://localhost:9901/stats/prometheus>

### Stop Metrics Services

To stop all services including the metrics stack:

```bash
docker compose --profile metrics down
```

To completely remove metrics data:

```bash
docker compose --profile metrics down -v
```

This removes the `prometheus-data` volume containing all stored metrics.

## Viewing Metrics in Grafana

Once you've started the gateway with the metrics profile, follow these steps to view component metrics:

### Step 1: Access Grafana

Open your browser and navigate to: <http://localhost:3000>

### Step 2: Log in to Grafana

1. Username: `admin`
2. Password: `admin`

**Note**: You'll be prompted to change the password on first login.

### Step 3: Navigate to Dashboards

1. Click on the **hamburger menu (☰)** in the top-left corner
2. Navigate to **Dashboards** → **Browse**
3. You'll see several pre-built dashboards:
   - **Infrastructure Overview**: High-level view of all components
   - **Gateway Controller**: Detailed gateway-controller metrics
   - **Policy Engine**: Detailed policy-engine metrics

### Step 4: View Infrastructure Overview

The Infrastructure Overview dashboard provides a comprehensive view:

#### Gateway Controller Section
- **API Operations**: Total operations and operation rate
- **Deployment Latency**: End-to-end deployment time
- **xDS Clients**: Number of connected Envoy routers
- **Database Operations**: Database operation metrics
- **HTTP Requests**: REST API request metrics

#### Policy Engine Section
- **Request Processing**: Total requests and request rate
- **Policy Executions**: Policy execution metrics
- **Active Streams**: Current ext_proc streams
- **Errors**: Error rate and types

#### System Resources
- **Memory Usage**: Heap, system memory across components
- **Goroutines**: Go runtime goroutines count
- **Uptime**: Component availability

### Step 5: View Gateway Controller Dashboard

The Gateway Controller dashboard provides detailed metrics:

#### API Management
- **API Operations Total**: Counter for all API operations with labels for:
  - `operation`: create, update, delete, get
  - `status`: success, failure
  - `api_type`: REST, GraphQL, etc.
- **APIs Total**: Gauge showing deployed APIs by type and status
- **Deployment Latency Seconds**: Histogram of deployment times

#### xDS Metrics
- **xDS Clients Connected**: Gauge of connected Envoy instances
- **Snapshot Generation Duration**: Time to generate configuration snapshots
- **XDS Stream Requests**: Counter for xDS requests by type
- **Snapshot Size**: Size of generated configuration snapshots

#### Database Metrics
- **Database Operations Total**: Counter for database operations
- **Database Operation Duration**: Histogram of operation times
- **Database Size Bytes**: Current database size

#### HTTP API Metrics
- **HTTP Requests Total**: Counter for REST API requests
- **HTTP Request Duration**: Histogram of API response times
- **Concurrent Requests**: Current concurrent API requests

### Step 6: View Policy Engine Dashboard

The Policy Engine dashboard provides detailed metrics:

#### Request Processing
- **Requests Total**: Counter for all processed requests with labels:
  - `phase`: request, response
  - `route`: route name
  - `api_name`: API identifier
  - `api_version`: API version
- **Request Duration Seconds**: Histogram of request processing times
- **Request Errors Total**: Counter for errors by type

#### Policy Execution
- **Policy Executions Total**: Counter for policy executions with labels:
  - `policy_name`: Name of executed policy
  - `policy_version`: Policy version
  - `api`: API identifier
  - `route`: Route name
  - `status`: success, failure, skip
- **Policy Duration Seconds**: Histogram of policy execution times
- **Policies Per Chain**: Gauge of current policy chain lengths

#### Streaming
- **Active Streams**: Current ext_proc streams (gauge)
- **XDS Updates Total**: Counter for configuration updates
- **Body Bytes Processed**: Counter for body processing

#### System Resources
- **Memory Usage**: Memory consumption metrics
- **Goroutines**: Current goroutines count
- **GRPC Connections**: Active gRPC connections

### Step 7: Create Custom Dashboards

You can create custom dashboards in Grafana:

1. Click **+** → **Dashboard**
2. Click **+ Add visualization**
3. Select Prometheus as the data source
4. Write PromQL queries to fetch metrics
5. Configure visualization (graphs, tables, gauges, etc.)
6. Save the dashboard

### Step 8: Set Up Alerts

Create alerts to be notified of issues:

1. Navigate to **Alerting** → **Alert rules**
2. Click **+ New alert rule**
3. Define the alert condition using PromQL
4. Set severity (Critical, Warning, Info)
5. Configure notifications (email, Slack, PagerDuty, etc.)
6. Save the alert rule

Example alert for high error rate:
```promql
(
  rate(gateway_controller_api_operations_total{status="failure"}[5m])
  /
  rate(gateway_controller_api_operations_total[5m])
) > 0.1
```

This alert triggers when the error rate exceeds 10% over 5 minutes.

## Prometheus Queries

You can query Prometheus directly at <http://localhost:9092> to create custom visualizations or debug issues.

### Useful Queries

#### Gateway Controller

**Total API Operations**:
```promql
rate(gateway_controller_api_operations_total[5m])
```

**API Operations by Status**:
```promql
rate(gateway_controller_api_operations_total[5m]) by (status)
```

**Deployment Latency Percentiles**:
```promql
histogram_quantile(0.95, rate(gateway_controller_deployment_latency_seconds_bucket[5m]))
```

**xDS Clients Connected**:
```promql
gateway_controller_xds_clients_connected
```

**Database Operation Rate**:
```promql
rate(gateway_controller_database_operations_total[5m])
```

**Memory Usage**:
```promql
gateway_controller_memory_bytes
```

**HTTP Request Duration**:
```promql
histogram_quantile(0.99, rate(gateway_controller_http_request_duration_seconds_bucket[5m]))
```

#### Policy Engine

**Request Rate**:
```promql
rate(policy_engine_requests_total[5m])
```

**Policy Execution Rate**:
```promql
rate(policy_engine_policy_executions_total[5m])
```

**Policy Execution Success Rate**:
```promql
rate(policy_engine_policy_executions_total{status="success"}[5m]) /
rate(policy_engine_policy_executions_total[5m])
```

**Average Request Duration**:
```promql
rate(policy_engine_request_duration_seconds_sum[5m]) /
rate(policy_engine_request_duration_seconds_count[5m])
```

**Active Streams**:
```promql
policy_engine_active_streams
```

**Error Rate**:
```promql
rate(policy_engine_request_errors_total[5m])
```

#### Router (Envoy)

**Request Rate**:
```promql
rate(envoy_http_internal_requests_total[5m])
```

**Request Duration**:
```promql
histogram_quantile(0.99, rate(envoy_http_request_duration_seconds_bucket[5m]))
```

**Upstream 5xx Errors**:
```promql
rate(envoy_http_upstream_rq_xx{envoy_response_flags="upstream_connect_fail"}[5m])
```

## Configuration Options

### Adjusting Scrape Interval

To reduce metrics collection overhead or increase granularity, adjust the Prometheus scrape interval:

Edit `gateway/observability/prometheus/prometheus.yml`:

```yaml
global:
  scrape_interval: 15s  # Change to 5s for higher granularity, 60s for lower overhead
  evaluation_interval: 15s
```

Restart Prometheus after changes:
```bash
docker compose restart prometheus
```

### Custom Metrics Endpoints

You can add additional metrics endpoints to scrape from:

```yaml
scrape_configs:
  - job_name: 'custom-service'
    static_configs:
      - targets: ['custom-service:8080']
    metrics_path: /metrics
    scrape_interval: 30s
```

### Metric Retention

Configure how long Prometheus retains metrics:

```yaml
command:
  - '--config.file=/etc/prometheus/prometheus.yml'
  - '--storage.tsdb.path=/prometheus'
  - '--web.enable-lifecycle'
  - '--storage.tsdb.retention.time=30d'  # Keep metrics for 30 days
  - '--storage.tsdb.retention.size=10GB'  # Keep up to 10GB of metrics
```

### Custom Bucket Configuration

Gateway components use optimized bucket configurations for histograms:

**Request Duration Buckets**:
- Gateway Controller: `[0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0]`
- Policy Engine: `[0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5]`

**Policy Execution Buckets**:
- `[0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5]`

**Deployment Latency Buckets**:
- `[0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0]`

## Alternative Metrics Backends

While the default setup uses Prometheus and Grafana, the gateway components expose standard Prometheus metrics and can integrate with any Prometheus-compatible system.

### AWS CloudWatch

Use AWS Distro for OpenTelemetry (ADOT) to export metrics to CloudWatch:

```yaml
adot-collector:
  image: public.ecr.aws/aws-observability/aws-otel-collector:latest
  command: ["--config=/etc/otel-collector-config.yaml"]
  environment:
    - AWS_REGION=us-east-1
```

Configure ADOT collector to scrape Prometheus metrics and export to CloudWatch.

### Datadog

Use Datadog Agent to scrape Prometheus metrics:

```yaml
datadog-agent:
  image: datadog/agent:latest
  environment:
    - DD_API_KEY=${DD_API_KEY}
    - DD_METRICS_SCRAPER_ENABLED=true
    - DD_SCRAPE_SERVICE_CHECKS=true
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock:ro
  networks:
    - gateway-network
```

Configure Datadog to scrape gateway endpoints:
```yaml
instances:
  - prometheus_url: http://gateway-controller:9091/metrics
    namespace: gateway
    metrics:
      - gateway_controller_*
  - prometheus_url: http://policy-engine:9003/metrics
    namespace: policy_engine
    metrics:
      - policy_engine_*
```

### New Relic

Use New Relic's Prometheus remote write integration:

```yaml
prometheus:
  image: prom/prometheus:latest
  command:
    - '--config.file=/etc/prometheus/prometheus.yml'
    - '--remote.write.url=https://metric-api.newrelic.com/prometheus/v1/write?account_id=YOUR_ACCOUNT_ID'
    - '--remote.write.headers=X-Api-Key:YOUR_API_KEY'
```

### InfluxDB

Use Prometheus remote write to send metrics to InfluxDB:

```yaml
prometheus:
  image: prom/prometheus:latest
  command:
    - '--config.file=/etc/prometheus/prometheus.yml'
    - '--remote.write.url=http://influxdb:8086/api/v1/prom/write?db=prometheus'
```

### Elasticsearch

Use Metricbeat to ship Prometheus metrics to Elasticsearch:

```yaml
metricbeat:
  image: elastic/metricbeat:latest
  volumes:
    - ./metricbeat.yml:/usr/share/metricbeat/metricbeat.yml:ro
  environment:
    - ELASTICSEARCH_HOST=elasticsearch:9200
```

Configure Metricbeat to scrape Prometheus endpoints.

### Azure Monitor

Use Azure Monitor Agent with Prometheus scraping:

```yaml
azuremonitor-agent:
  image: mcr.microsoft.com/azuremonitor/metrics-adapter:latest
  environment:
    - AZURE_CLIENT_ID=${AZURE_CLIENT_ID}
    - AZURE_TENANT_ID=${AZURE_TENANT_ID}
    - AZURE_CLIENT_SECRET=${AZURE_CLIENT_SECRET}
```

### Google Cloud Monitoring

Use Cloud Monitoring Prometheus sidecar:

```yaml
prometheus-to-monitoring:
  image: gcr.io/cloud-prometheus/prometheus-to-monitoring:latest
  environment:
    - GOOGLE_APPLICATION_CREDENTIALS=/var/secrets/google/key.json
  volumes:
    - ./key.json:/var/secrets/google/key.json:ro
```

### Grafana Cloud

Use Prometheus remote write to Grafana Cloud:

```yaml
prometheus:
  image: prom/prometheus:latest
  command:
    - '--config.file=/etc/prometheus/prometheus.yml'
    - '--remote.write.url=https://YOUR-PROMETHEUS-URL/api/v1/write'
    - '--remote.write.headers=Authorization:Bearer YOUR-API-KEY'
```

### VictoriaMetrics

VictoriaMetrics is a Prometheus-compatible time-series database:

```yaml
victoriametrics:
  image: victoriametrics/victoria-metrics:latest
  ports:
    - "8428:8428"
  volumes:
    - victoriametrics-data:/victoria-metrics-data
```

Configure Prometheus to remote write to VictoriaMetrics:
```yaml
prometheus:
  image: prom/prometheus:latest
  command:
    - '--config.file=/etc/prometheus/prometheus.yml'
    - '--remote.write.url=http://victoriametrics:8428/api/v1/write'
```

### Thanos

Thanos provides long-term storage and global query view for Prometheus:

```yaml
thanos-store:
  image: thanosio/thanos:latest
  ports:
    - "10901:10901"
  volumes:
    - prometheus-data:/prometheus

thanos-query:
  image: thanosio/thanos:latest
  ports:
    - "10902:10902"
  command:
    - 'query'
    - '--store=thanos-store:10901'
```

Configure Prometheus to upload blocks to Thanos object storage.

## Metric Reference

### Gateway Controller Metrics

#### API Management
- `gateway_controller_api_operations_total`: Counter of API operations
  - Labels: `operation`, `status`, `api_type`
- `gateway_controller_api_operation_duration_seconds`: Histogram of operation duration
  - Labels: `operation`, `api_type`
- `gateway_controller_apis_total`: Gauge of deployed APIs
  - Labels: `api_type`, `status`
- `gateway_controller_validation_errors_total`: Counter of validation errors
  - Labels: `operation`, `error_type`
- `gateway_controller_deployment_latency_seconds`: Histogram of deployment latency

#### xDS Metrics
- `gateway_controller_xds_clients_connected`: Gauge of connected xDS clients
  - Labels: `server`, `node_id`
- `gateway_controller_snapshot_generation_duration_seconds`: Histogram of snapshot generation time
  - Labels: `type`
- `gateway_controller_snapshot_generation_total`: Counter of snapshot generations
  - Labels: `type`, `status`, `trigger`
- `gateway_controller_snapshot_size`: Gauge of snapshot resource size
  - Labels: `resource_type`
- `gateway_controller_xds_stream_requests_total`: Counter of xDS stream requests
  - Labels: `server`, `type_url`, `operation`
- `gateway_controller_xds_snapshot_ack_total`: Counter of snapshot ACK/NACK
  - Labels: `server`, `node_id`, `status`

#### Database Metrics
- `gateway_controller_database_operations_total`: Counter of database operations
  - Labels: `operation`, `table`, `status`
- `gateway_controller_database_operation_duration_seconds`: Histogram of operation duration
  - Labels: `operation`, `table`
- `gateway_controller_database_size_bytes`: Gauge of database size
  - Labels: `database`
- `gateway_controller_config_store_size`: Gauge of config store items
  - Labels: `type`

#### HTTP API Metrics
- `gateway_controller_http_requests_total`: Counter of HTTP requests
  - Labels: `method`, `endpoint`, `status_code`
- `gateway_controller_http_request_duration_seconds`: Histogram of request duration
  - Labels: `method`, `endpoint`
- `gateway_controller_http_request_size_bytes`: Histogram of request size
  - Labels: `endpoint`
- `gateway_controller_http_response_size_bytes`: Histogram of response size
  - Labels: `endpoint`
- `gateway_controller_concurrent_requests`: Gauge of concurrent requests

#### System Metrics
- `gateway_controller_up`: Gauge of component liveness (1=up, 0=down)
- `gateway_controller_info`: Gauge of build information
  - Labels: `version`, `storage_type`, `build_date`
- `gateway_controller_goroutines`: Gauge of current goroutines
- `gateway_controller_memory_bytes`: Gauge of memory usage
  - Labels: `type` (heap_alloc, heap_sys, stack_inuse)

#### Error Metrics
- `gateway_controller_errors_total`: Counter of errors
  - Labels: `component`, `error_type`
- `gateway_controller_panic_recoveries_total`: Counter of panic recoveries
  - Labels: `component`
- `gateway_controller_storage_errors_total`: Counter of storage errors
  - Labels: `operation`, `error_type`
- `gateway_controller_translation_errors_total`: Counter of translation errors
  - Labels: `error_type`

#### Certificate Metrics
- `gateway_controller_certificates_total`: Gauge of certificates
  - Labels: `type`
- `gateway_controller_certificate_operations_total`: Counter of certificate operations
  - Labels: `operation`, `status`
- `gateway_controller_certificate_expiry_seconds`: Gauge of certificate expiry
  - Labels: `cert_id`, `cert_name`

#### Policy Metrics
- `gateway_controller_policies_total`: Gauge of policies
  - Labels: `api_id`, `route`
- `gateway_controller_policy_chain_length`: Histogram of policy chain length
  - Labels: `api_id`, `route`
- `gateway_controller_policy_snapshot_updates_total`: Counter of policy updates
  - Labels: `status`
- `gateway_controller_policy_validation_errors_total`: Counter of validation errors
  - Labels: `error_type`

### Policy Engine Metrics

#### Request Processing
- `policy_engine_requests_total`: Counter of processed requests
  - Labels: `phase` (request, response), `route`, `api_name`, `api_version`
- `policy_engine_request_duration_seconds`: Histogram of request duration
  - Labels: `phase`, `route`
- `policy_engine_request_errors_total`: Counter of request errors
  - Labels: `phase`, `error_type`, `route`
- `policy_engine_short_circuits_total`: Counter of short-circuited requests
  - Labels: `route`, `policy_name`

#### Policy Execution
- `policy_engine_policy_executions_total`: Counter of policy executions
  - Labels: `policy_name`, `policy_version`, `api`, `route`, `status`
- `policy_engine_policy_duration_seconds`: Histogram of policy execution duration
  - Labels: `policy_name`, `policy_version`, `api`, `route`
- `policy_engine_policy_skipped_total`: Counter of skipped policies
  - Labels: `policy_name`, `api`, `route`, `reason`
- `policy_engine_policies_per_chain`: Gauge of current policies per chain
  - Labels: `route`, `api`

#### Configuration
- `policy_engine_policy_chains_loaded`: Gauge of loaded policy chains
  - Labels: `mode` (file, xds)
- `policy_engine_xds_updates_total`: Counter of xDS updates
  - Labels: `status`, `type`
- `policy_engine_xds_connection_state`: Gauge of xDS connection state
  - Labels: `state`
- `policy_engine_snapshot_size`: Gauge of snapshot size
  - Labels: `resource_type`

#### Streaming
- `policy_engine_active_streams`: Gauge of active ext_proc streams
- `policy_engine_body_bytes_processed`: Counter of body bytes processed
  - Labels: `phase`, `operation`
- `policy_engine_context_build_duration_seconds`: Histogram of context build duration
  - Labels: `type`
- `policy_engine_grpc_connections_active`: Gauge of active gRPC connections
  - Labels: `type`

#### System Metrics
- `policy_engine_up`: Gauge of component liveness (1=up, 0=down)
- `policy_engine_goroutines`: Gauge of current goroutines
- `policy_engine_memory_bytes`: Gauge of memory usage
  - Labels: `type` (heap_alloc, heap_sys, stack)

#### Error Metrics
- `policy_engine_policy_errors_total`: Counter of policy errors
  - Labels: `policy_name`, `error_type`
- `policy_engine_stream_errors_total`: Counter of stream errors
  - Labels: `error_type`
- `policy_engine_route_lookup_failures_total`: Counter of route lookup failures
- `policy_engine_panic_recoveries_total`: Counter of panic recoveries
  - Labels: `component`

### Router (Envoy) Metrics

Envoy exposes built-in Prometheus metrics. Key metrics include:

#### HTTP Metrics
- `envoy_http_internal_requests_total`: Counter of HTTP requests
  - Labels: `virtual_cluster`, `virtual_host`, `response_code`
- `envoy_http_request_duration_seconds`: Histogram of request duration
- `envoy_http_downstream_cx_active`: Gauge of active connections
- `envoy_http_downstream_cx_total`: Counter of connections

#### Upstream Metrics
- `envoy_http_upstream_rq_total`: Counter of upstream requests
  - Labels: `upstream_cluster`, `response_code`
- `envoy_http_upstream_rq_xx`: Counter of upstream requests by status
  - Labels: `upstream_cluster`, `envoy_response_flags`

#### Cluster Metrics
- `envoy_cluster_upstream_cx_active`: Gauge of active upstream connections
- `envoy_cluster_upstream_rq_retry_total`: Counter of retry requests
- `envoy_cluster_membership_healthy`: Gauge of healthy endpoints

#### Listener Metrics
- `envoy_listener_downstream_cx_active`: Gauge of active downstream connections
- `envoy_listener_downstream_cx_total`: Counter of downstream connections

## Integration with Logging and Tracing

Metrics, logs, and traces work together for comprehensive observability:

### Correlating Metrics with Logs

1. **Correlation IDs**: Gateway components include request IDs in both logs and metrics
2. **Find Logs from Metrics**: Use metric labels to filter logs (e.g., API name, route)
3. **Find Metrics from Logs**: Copy request IDs from logs and query metrics with labels

Example: If you see errors in logs for a specific API, query metrics for that API:
```promql
gateway_controller_api_operations_total{api_name="Weather-API", status="failure"}
```

### Correlating Metrics with Traces

Metrics and traces share labels for correlation:
- Trace IDs are included in log entries
- You can search traces by API name or route
- Span attributes include metric labels

### Using All Three Stacks

Enable all observability profiles:
```bash
docker compose --profile logging --profile tracing --profile metrics up -d
```

This provides:
- **Metrics**: Quantitative measurements and alerting
- **Traces**: Request flow and performance debugging
- **Logs**: Detailed event information and error context

## Best Practices

### Development
- Use default scrape interval (15s) for reasonable granularity
- Keep short retention (7-15 days) to save disk space
- Enable debug logging for troubleshooting
- Use Grafana dashboards for real-time monitoring

### Production
- **Adjust scrape intervals** based on traffic:
  - Low traffic (<100 req/s): 15s interval
  - Medium traffic (100-1000 req/s): 10s interval
  - High traffic (>1000 req/s): 5s interval
- **Configure appropriate retention**:
  - Short-term (hot): 7-30 days
  - Medium-term (warm): 90 days
  - Long-term (cold): 1+ years (use Thanos or remote write)
- **Set up alerts** for critical metrics:
  - Error rate > 5%
  - 95th percentile latency > 1s
  - Memory usage > 80%
  - Active streams approaching limit
- **Use recording rules** for frequently queried metrics:
  ```yaml
  groups:
    - name: recording_rules
      interval: 30s
      rules:
        - record: gateway:api_error_rate_5m
          expr: |
            rate(gateway_controller_api_operations_total{status="failure"}[5m]) /
            rate(gateway_controller_api_operations_total[5m])
  ```
- **Monitor Prometheus itself**:
  - Scrape duration
  - Rule evaluation time
  - Storage usage
  - Query performance

### Security
- **Restrict metrics endpoints** in production
- **Enable authentication** for Grafana
- **Use TLS** for metrics endpoints (if exposed externally)
- **Sanitize sensitive data** from metrics
- **Implement access controls** for dashboards
- **Regularly audit** dashboard and alert permissions

### Performance
- **Optimize PromQL queries**:
  - Use rate() for counters over time ranges
  - Use histogram_quantile() for percentiles
  - Avoid high-cardinality labels (e.g., user IDs)
- **Use recording rules** for expensive queries
- **Limit dashboard refresh rates** (30s minimum)
- **Prune unused metrics** to reduce cardinality
- **Compress metric names** to reduce storage

### Metric Cardinality

Avoid high-cardinality labels (millions of unique values):

**Good** (low cardinality):
```promql
gateway_controller_api_operations_total{route="/weather/v1"}
```

**Bad** (high cardinality):
```promql
gateway_controller_api_operations_total{user_id="12345"}
```

### Query Optimization

**Use time ranges**:
```promql
# Bad: No time range (prometheus returns default)
gateway_controller_api_operations_total

# Good: Explicit rate over 5 minutes
rate(gateway_controller_api_operations_total[5m])
```

**Use subqueries efficiently**:
```promql
# Bad: Outer query has range, inner query has range
rate(rate(gateway_controller_http_request_duration_seconds_sum[5m])[10m:1m])

# Good: Single rate call
rate(gateway_controller_http_request_duration_seconds_sum[5m])
```

## Troubleshooting

### Metrics Not Appearing in Grafana

**1. Verify metrics are enabled in configuration:**
```bash
grep -A5 "metrics:" gateway/configs/config.yaml
```

Ensure `enabled: true`.

**2. Check Prometheus is running:**
```bash
docker ps | grep prometheus
curl http://localhost:9092/-/healthy
```

**3. Verify Prometheus configuration:**
```bash
docker exec prometheus cat /etc/prometheus/prometheus.yml
```

**4. Check Prometheus targets:**
- Navigate to <http://localhost:9092/targets>
- Verify all endpoints are "UP" (green)
- If endpoints are "DOWN", check:
  - Container is running
  - Port is accessible from Prometheus container
  - Metrics endpoint is responding

**5. Test metrics endpoint directly:**
```bash
curl http://localhost:9091/metrics | head -20  # Gateway Controller
curl http://localhost:9003/metrics | head -20  # Policy Engine
```

**6. Check Grafana data source:**
- Navigate to <http://localhost:3000/connections/datasources>
- Verify Prometheus data source is configured
- Test connection should succeed

**7. Verify network connectivity:**
```bash
docker exec prometheus wget -O- gateway-controller:9091/metrics
docker exec prometheus wget -O- policy-engine:9003/metrics
```

### High Cardinality Metrics

**Symptoms:**
- Prometheus memory usage constantly increasing
- Slow query performance
- Many unique label value combinations

**Diagnosis:**
```bash
# Check metric cardinality
curl http://localhost:9091/metrics | wc -l  # Count metric lines
```

**Solutions:**
- Remove high-cardinality labels (user IDs, session IDs, etc.)
- Use histogram buckets instead of labels
- Aggregate before labeling

### Missing Metrics

**1. Check if metric name changed** (after component update)
```bash
curl http://localhost:9091/metrics | grep "gateway_controller_" | head -20
```

**2. Verify metrics are being scraped**
```promql
# Prometheus query to check if metric exists
up{job="gateway-controller"}
```

**3. Check component logs for metrics errors:**
```bash
docker logs gateway-controller | grep -i metric
docker logs policy-engine | grep -i metric
```

### Grafana Dashboards Not Loading

**1. Verify Grafana is running:**
```bash
docker ps | grep grafana
curl http://localhost:3000/api/health
```

**2. Check Grafana logs:**
```bash
docker logs grafana
```

**3. Verify data source configuration:**
- Navigate to <http://localhost:3000/connections/datasources>
- Check Prometheus URL: `http://prometheus:9090`
- Test connection

**4. Clear browser cache and reload dashboard**

**5. Re-import dashboards:**
```bash
# Navigate to Dashboards → Import
# Upload JSON files from ./observability/grafana/dashboards/
```

### High Memory Usage

**1. Check Prometheus memory usage:**
```bash
docker stats prometheus
```

**2. Review retention settings:**
```bash
docker exec prometheus cat /etc/prometheus/prometheus.yml | grep retention
```

**3. Check metric cardinality:**
```promql
# Count unique metric label combinations
count by (__name__) ({__name__=~".+"})
```

**4. Reduce retention:**
```yaml
command:
  - '--storage.tsdb.retention.time=7d'
  - '--storage.tsdb.retention.size=2GB'
```

**5. Use Thanos or remote write** for long-term storage

### Slow Queries

**1. Identify slow queries:**
```promql
# Check query duration in Prometheus UI
# Navigate to http://localhost:9092/graph
# Run query and check execution time
```

**2. Optimize queries:**
- Use rate() instead of raw counters
- Use proper time ranges
- Avoid high-cardinality labels
- Use recording rules for common queries

**3. Increase Prometheus resources:**
```yaml
prometheus:
  image: prom/prometheus:latest
  deploy:
    resources:
      limits:
        memory: 4G
        cpus: '2'
```

### Metric Values Not Updating

**1. Check if metrics are counters with rate:**
```promql
# Counter without rate (shows cumulative total)
gateway_controller_api_operations_total

# Counter with rate (shows rate of change)
rate(gateway_controller_api_operations_total[5m])
```

**2. Verify scrape configuration:**
```bash
docker exec prometheus cat /etc/prometheus/prometheus.yml | grep -A5 job_name
```

**3. Check component is receiving traffic:**
```promql
rate(gateway_controller_api_operations_total[5m]) > 0
```

## Disabling Metrics

To completely disable metrics:

1. **Update configuration** in `gateway/configs/config.yaml`:

```yaml
gateway_controller:
  metrics:
    enabled: false

policy_engine:
  metrics:
    enabled: false
```

2. **Restart gateway services:**

```bash
docker compose restart gateway-controller policy-engine router
```

**Note:** Disabling metrics will:
- Stop HTTP metrics servers on the configured ports
- Remove metrics from Prometheus targets
- No new metric data will be collected

## Additional Resources

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [Prometheus Querying Guide](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Grafana Dashboard Tutorial](https://grafana.com/docs/grafana/latest/dashboards/)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/naming/)
- [Gateway README](../README.md)
- [Gateway Logging Guide](logging.md)
- [Gateway Tracing Guide](tracing.md)
- [Docker Compose Profiles](https://docs.docker.com/compose/profiles/)
