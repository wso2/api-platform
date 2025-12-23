# Gateway Logging

This guide explains how to implement and configure logging for the API Platform Gateway components.

## Overview

The default logging services included in the Docker Compose configuration are **demonstration services** designed to showcase how you can observe component logs in a centralized setup. These services provide a reference implementation that you can use out-of-the-box for development, testing, or as a starting point for your production logging strategy.

**Important**: You are free to choose any logging or observability strategy that suits your environment and requirements. The provided setup is just one of many possible configurations.

## Logging Architecture

The default logging stack consists of:

- **Fluent Bit**: Lightweight log collector that reads Docker container logs and forwards them to OpenSearch
- **OpenSearch**: Stores and indexes log data for searchability and analysis
- **OpenSearch Dashboards**: Web interface for visualizing, exploring, and searching logs

### How It Works

1. Gateway components (gateway-controller, policy-engine, router) write structured JSON logs to stdout/stderr
2. Docker captures these logs and stores them in `/var/lib/docker/containers`
3. Fluent Bit tails these log files, parses them, and enriches them with metadata (component name, hostname)
4. Fluent Bit forwards processed logs to OpenSearch
5. Users can view and search logs through OpenSearch Dashboards

## Enabling Logging Services

### Gateway Components Already Log to Standard Output

**No special configuration is required to enable logging in the gateway components.** All gateway components (gateway-controller, policy-engine, and router) follow the [12-factor app](https://12factor.net/logs) architecture principle for logging:

- Components write all logs to **stdout (standard output)** and **stderr (standard error)**
- Logs are emitted as structured JSON for easy parsing
- No file-based logging or log management is built into the components

This architecture approach allows you to utilize **any industry-standard logging stack** to collect logs from Docker container log files and view them in your preferred observability platform. The gateway components are completely decoupled from the logging infrastructure.

### Demonstrated Logging Services

The logging services included in the Docker Compose file (OpenSearch, OpenSearch Dashboards, and Fluent Bit) are provided as **demonstration services** to show one possible way to collect and visualize logs. You can use them as-is for development/testing, or replace them with your own logging solution.

The gateway uses Docker Compose profiles to optionally enable these demonstration logging services.

### Start Gateway with Demonstrated Logging Services

To start the gateway with the demonstration logging services enabled:

```bash
docker compose --profile logging up -d
```

This starts:
- Core gateway services (gateway-controller, policy-engine, router) - *which log to stdout/stderr*
- OpenSearch - *stores and indexes logs*
- OpenSearch Dashboards - *web UI for viewing logs*
- Fluent Bit - *collects logs from Docker and forwards to OpenSearch*

### Start Gateway without Logging Services

To run only the core gateway services without the demonstration logging stack:

```bash
docker compose up -d
```

**Note:** The gateway components still log to stdout/stderr. You just won't have the centralized collection and visualization services running. You can still view logs using:

```bash
docker logs gateway-controller
docker logs policy-engine
docker logs router
```

### Stop Logging Services

To stop all services including the logging stack:

```bash
docker compose --profile logging down
```

To completely remove logging data:

```bash
docker compose --profile logging down -v
```

This removes the `opensearch-data` volume containing all stored logs.

## Viewing Logs in OpenSearch Dashboards

Once you've started the gateway with the logging profile, follow these steps to view component logs:

### Step 1: Access OpenSearch Dashboards

Open your browser and navigate to:
```
http://localhost:5601
```

### Step 2: Create an Index Pattern

Before you can view logs, you need to create an index pattern:

1. Click on the **hamburger menu (☰)** in the top-left corner
2. Navigate to **Management** → **Dashboard Management**
3. Under **Dashboard Management**, click **Index Patterns**
4. Click **Create index pattern**
5. Enter the index pattern: `gateway-logs-*`
6. Click **Next step**
7. Select **@timestamp** as the time field
8. Click **Create index pattern**

### Step 3: Navigate to Discover

To view and explore logs:

1. Click the **hamburger menu (☰)**
2. Navigate to **OpenSearch Dashboards** → **Discover**
3. Select the `gateway-logs-*` index pattern from the dropdown in the top-left
4. Adjust the time range in the top-right corner if needed (default is last 15 minutes)

### Step 4: Filter Logs by Component

To view logs for a specific gateway component, use filters:

#### View Policy Engine Logs

1. Click **Add filter** (below the search bar)
2. **Field**: Select `component`
3. **Operator**: Select `is`
4. **Value**: Enter `policy-engine`
5. Click **Save**

#### View Gateway Controller Logs

1. Click **Add filter**
2. **Field**: `component`
3. **Operator**: `is`
4. **Value**: `gateway-controller`
5. Click **Save**

#### View Router (Envoy) Logs

1. Click **Add filter**
2. **Field**: `component`
3. **Operator**: `is`
4. **Value**: `router`
5. Click **Save**

### Step 5: Search and Filter Logs

You can refine your log search using:

#### Free Text Search
Enter keywords in the search bar at the top:
```
error
```
```
Weather-API
```

#### Filter by Log Level
1. Click **Add filter**
2. **Field**: `level`
3. **Operator**: `is`
4. **Value**: `error` (or `info`, `warn`, `debug`)

#### Combine Multiple Filters
Add multiple filters to narrow down results. For example:
- Component: `policy-engine`
- Level: `error`
- Time range: Last 1 hour

#### Example Search Queries

Search for errors in the policy engine:
```
component:policy-engine AND level:error
```

Search for specific API logs:
```
component:gateway-controller AND message:*Weather-API*
```

Search for slow requests (if duration field exists):
```
duration_ms:>1000
```

### Step 6: Customize Log View

- **Add/Remove Columns**: Click the **gear icon** next to the field list to select which fields to display
- **Sort**: Click column headers to sort by that field
- **Expand Logs**: Click the **>** arrow next to any log entry to see full details in JSON format
- **Save Search**: Click **Save** in the top menu to save your filters and queries for later use

## Alternative Logging Stacks

While the default setup uses OpenSearch and Fluent Bit, you can integrate with other logging platforms:

### Elastic Stack (ELK)

Replace OpenSearch with the Elastic Stack:

```yaml
elasticsearch:
  image: docker.elastic.co/elasticsearch/elasticsearch:8.11.0
  environment:
    - discovery.type=single-node
    - xpack.security.enabled=false
  ports:
    - "9200:9200"
  networks:
    - gateway-network

kibana:
  image: docker.elastic.co/kibana/kibana:8.11.0
  environment:
    - ELASTICSEARCH_HOSTS=http://elasticsearch:9200
  ports:
    - "5601:5601"
  networks:
    - gateway-network
  depends_on:
    - elasticsearch
```

Update Fluent Bit output:
```conf
[OUTPUT]
    Name es
    Match docker.*
    Host elasticsearch
    Port 9200
    Logstash_Format On
    Logstash_Prefix gateway-logs
```

### Grafana Loki

For a lightweight, Prometheus-inspired logging solution:

```yaml
loki:
  image: grafana/loki:latest
  ports:
    - "3100:3100"
  command: -config.file=/etc/loki/local-config.yaml
  networks:
    - gateway-network

promtail:
  image: grafana/promtail:latest
  volumes:
    - /var/lib/docker/containers:/var/lib/docker/containers:ro
    - ./observability/promtail/config.yaml:/etc/promtail/config.yaml:ro
  command: -config.file=/etc/promtail/config.yaml
  networks:
    - gateway-network

grafana:
  image: grafana/grafana:latest
  ports:
    - "3000:3000"
  networks:
    - gateway-network
  depends_on:
    - loki
```

### Cloud-Native Solutions

#### AWS CloudWatch

Configure Fluent Bit to send logs to CloudWatch:

```conf
[OUTPUT]
    Name cloudwatch_logs
    Match *
    region us-east-1
    log_group_name /aws/gateway
    log_stream_prefix gateway-
    auto_create_group true
```

Add AWS credentials via environment variables or IAM roles.

#### Datadog

Use the Datadog Agent:

```yaml
datadog:
  image: datadog/agent:latest
  environment:
    - DD_API_KEY=${DD_API_KEY}
    - DD_LOGS_ENABLED=true
    - DD_LOGS_CONFIG_CONTAINER_COLLECT_ALL=true
    - DD_AC_EXCLUDE=name:datadog-agent
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock:ro
    - /var/lib/docker/containers:/var/lib/docker/containers:ro
  networks:
    - gateway-network
```

#### Splunk

Configure Fluent Bit to forward to Splunk HEC:

```conf
[OUTPUT]
    Name splunk
    Match *
    Host splunk.example.com
    Port 8088
    Splunk_Token ${SPLUNK_HEC_TOKEN}
    TLS On
    TLS.Verify Off
```


## Additional Resources

- [Fluent Bit Documentation](https://docs.fluentbit.io/)
- [OpenSearch Documentation](https://opensearch.org/docs/latest/)
- [OpenSearch Dashboards User Guide](https://opensearch.org/docs/latest/dashboards/)
- [Gateway README](README.md)
- [Docker Compose Profiles](https://docs.docker.com/compose/profiles/)
