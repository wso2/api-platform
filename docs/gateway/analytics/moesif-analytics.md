# Analytics

## Overview

The Analytics feature enables the API Platform to capture, process, and publish API request and response data for observability and business insights. Analytics data is collected asynchronously from the gateway without impacting request latency and is published to an external analytics platform for further analysis and visualization.

This capability allows platform administrators and business stakeholders to gain visibility into API usage patterns, traffic behavior, latency characteristics, and consumer activity across the platform.


## Features

* Asynchronous collection of API request and response data
* Policy-enriched analytics metadata capture
* Zero impact on request/response latency
* Batched and configurable publishing to external analytics platforms
* Horizontally scalable analytics processing pipeline
* Pluggable publisher model (supports multiple analytics backends)


## Prerequisites

 - Active Moesif Account and an Application ID
> **Note:** For obtaining the Application ID:
> - Step 1: Sign up in [Moesif](https://www.moesif.com/)
> - Sept 2: Follow the onboarding wizard.
> - Sept 3: During the sign up process, you will receive a Collector Application ID for your configured application. Copy this value and keep it saved.

> For more detailed instructions and advanced configuration options, refer to the [official Moesif Documentation](https://www.moesif.com/docs).


## Configuration

Analytics is configured entirely through the gateway `config.toml` file and is enabled at a system level.

Analytics is a *consumer* of the shared `[collector]` capture pipeline: enabling `analytics.enabled` implicitly activates the collector too, since the collector has no on/off flag of its own. The collector captures request/response headers and bodies and ships them over an internal Envoy → policy-engine gRPC access-log (ALS) stream; the publisher(s) configured under `[analytics]` then deliver that data to the external analytics platform. Configure *what* gets captured and *how it's transported* under `[collector]` / `[collector.server]` (below); configure *where it's published* under `[analytics]`.

### System Parameters (`config.toml`)

#### Analytics

| Parameter            | Type    | Required | Default      | Description                                                                    |
| --------------------- | ------- | -------- | ------------ | -------------------------------------------------------------------------------- |
| `enabled`             | boolean | Yes      | `false`      | Enables or disables analytics globally                                          |
| `enabled_publishers`  | list    | No       | `["moesif"]` | Names of the publishers (configured under `publishers` below) that are active   |

#### Publishers

`publishers` is a table keyed by publisher name — currently only `moesif` is supported. A publisher's own settings are set directly under `[analytics.publishers.<name>]`; there is no separate `type`/`enabled`/`settings` wrapper.

For Moesif, the following attributes are supported under `[analytics.publishers.moesif]`:

| Parameter              | Type    | Required | Description                                |
| ---------------------- | ------- | -------- | ------------------------------------------- |
| `application_id`       | string  | Yes      | Moesif application/collector identifier    |
| `moesif_base_url`      | string  | No       | Override the Moesif API base URL           |
| `publish_interval`     | int     | Yes      | Interval (seconds) between publish cycles  |
| `event_queue_size`     | int     | Yes      | Maximum events held in memory              |
| `batch_size`           | int     | Yes      | Maximum events per batch                   |
| `timer_wakeup_seconds` | int     | Yes      | Publisher timer resolution                 |

#### Collector

The collector is the shared capture pipeline feeding analytics (and, independently, the stdout traffic-logging feature). It has no `enabled` flag of its own — it activates automatically whenever `analytics.enabled` (or `traffic_logging.enabled`) is `true`.

| Parameter              | Type    | Required | Default | Description                                                                |
| ----------------------- | ------- | -------- | ------- | ---------------------------------------------------------------------------- |
| `request_body`          | boolean | No       | `false` | Capture request bodies into the collected event                             |
| `response_body`         | boolean | No       | `false` | Capture response bodies into the collected event                            |
| `request_headers`       | boolean | No       | `false` | Capture all request headers into the collected event                        |
| `response_headers`      | boolean | No       | `false` | Capture all response headers into the collected event                       |
| `ignore_path_prefixes`  | list    | No       | `[]`    | Path prefixes for which no analytics/traffic-log event is produced at all   |

#### Collector Server (ALS transport)

`[collector.server]` tunes the Envoy → policy-engine gRPC access-log (ALS) stream that carries captured events. Both the Envoy-side sender configuration and the policy-engine's receiving ALS server read this same section.

| Parameter               | Type     | Required | Default | Description                      |
| ----------------------- | -------- | -------- |---- | -------------------------------- |
| `mode`                  | string   | No       | `uds`         | Connection mode: `uds` or `tcp` |
| `buffer_flush_interval` | int (ns) | No       | `1000000000` | Maximum time Envoy waits(in nanoseconds) before flushing buffered access log entries.|
| `buffer_size_bytes`     | int      | No       | `16384` | Maximum size of the in-memory buffer used to batch access log entries before sending them to the ALS server.                  |
| `grpc_request_timeout`  | int (ns) | No       | `20000000000` | Timeout duration Envoy waits(in nanoseconds) for a response from the ALS server before considering the log delivery attempt failed.            |
| `shutdown_timeout`      | duration | No       | `600s` | Maximum time allowed for the ALS server to gracefully shut down while completing in-flight log processing. |
| `als_plain_text`        | boolean | No       | `true` | Use plaintext gRPC        |
| `public_key_path`       | string | No       | - | Path to the public key used for securing ALS communication when transport-level encryption or authentication is enabled.        |
| `private_key_path`      | string | No       | - | Path to the private key used for securing ALS communication when transport-level encryption or authentication is enabled.        |
| `max_message_size`      | int     | No       | `1000000000` |Maximum size of a single gRPC message that the ALS server is allowed to receive from Envoy.      |
| `max_header_limit`      | int     | No       | `8192` | Maximum allowed size of request or response headers processed by the ALS server      |

**Note:** The hostname for the ALS connection is automatically derived from the policy-engine configuration. The internal log name identifier is set to `"envoy_access_log"` and is not configurable.

> **Deprecated:** `[analytics.grpc_event_server]` / `[analytics.access_logs_service]` and `analytics.allow_payloads` / `analytics.send_request_body` / `analytics.send_response_body` are older aliases for the settings above. They still work today (migrated onto `[collector]` / `[collector.server]` at startup, with a warning logged), but new configuration should use `[collector]` / `[collector.server]` directly. Setting both a deprecated alias and `[collector.server]` causes the deprecated block — including any TLS settings it configured — to be silently dropped in favor of `[collector.server]` as configured.


## Configuration Examples

### Integrate Moesif Publisher

For Moesif analytics integration, configure the following publisher-specific attributes under `[analytics.publishers.moesif]`. These parameters control authentication, batching behavior, and publish intervals for efficient analytics delivery.

`[collector]`'s capture flags are off by default, matching the gateway's own shipped default — enable them deliberately. There is no redaction at the collector layer

```toml
[analytics]
enabled = true
enabled_publishers = ["moesif"]

[analytics.publishers.moesif]
application_id = '{{ env "APIP_GW_ANALYTICS_PUBLISHERS_MOESIF_APPLICATION_ID" "" }}'
publish_interval = 5
event_queue_size = 10000
batch_size = 50
timer_wakeup_seconds = 3

[collector]
request_body = false
response_body = false
request_headers = false
response_headers = false

[collector.server]
mode = "uds"
buffer_flush_interval = 1000000000
buffer_size_bytes = 16384
grpc_request_timeout = 20000000000
shutdown_timeout = "600s"
als_plain_text = true
max_message_size = 1000000000
max_header_limit = 8192
```


## Use Cases

* **API Usage Visibility** – Understand how APIs are consumed across tenants and applications.
* **Operational Insights** – Observe traffic volume, response behavior, and latency trends.
* **Business Intelligence** – Support product and business decisions using API analytics data.
* **Platform Monitoring** – Gain observability into API behavior without impacting performance.

