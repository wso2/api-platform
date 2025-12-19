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

Analytics is configured entirely through the gateway `config.yaml` file and is enabled at a system level.

### System Parameters (`config.yaml`)

#### Analytics

| Parameter | Type    | Required | Default | Description                            |
| --------- | ------- | -------- |-------- | -------------------------------------- |
| `enabled` | boolean | Yes      | false   | Enables or disables analytics globally |

#### Publishers

| Parameter              | Type    | Required | Description                               |
| ---------------------- | ------- | -------- | ----------------------------------------- |
| `type`                 | string  | Yes      | Analytics publisher type (Currently limited only to ```moesif``` )                  |
| `enabled`              | boolean | Yes      | Enables the publisher                     |
| `settings`             | object | Yes       | Map of Publisher specific attributes required for configuring the publisher client                     |

#### gRPC Access Logs

| Parameter               | Type     | Required | Default | Description                      |
| ----------------------- | -------- | -------- |---- | -------------------------------- |
| `host`                  | string   | Yes      | - | Hostname or service name of the Access Log Service (ALS) server to which Envoy streams access logs over gRPC.              |
| `log_name`              | string   | No       | `"envoy_access_log"`| Logical identifier attached to the access log stream, used to distinguish log streams on the server side. |
| `buffer_flush_interval` | duration | No       | `1000000000`| Maximum time Envoy waits(in nanoseconds) before flushing buffered access log entries.|
| `buffer_size_bytes`     | int      | No       | `16384` | Maximum size of the in-memory buffer used to batch access log entries before sending them to ALS server.                  |
| `grpc_request_timeout`  | duration | No       | `20000000000` | Timeout duration Envoy waits(in nanoseconds) for a response from the ALS server before considering the log delivery attempt failed.            |

#### Access Log Service

| Parameter          | Type    | Required | Default | Description               |
| ------------------ | ------- | -------- | --- | ------------------------- |
| `als_server_port`  | int     | Yes      | - | gRPC port on which the ALS server listens for incoming access log streams from Envoy.      |
| `shutdown_timeout` | int     | No       | `600` | Maximum time allowed for the ALS server to gracefully shut down while completing in-flight log processing(in seconds). |
| `als_plain_text`   | boolean | No       | `true` | Use plaintext gRPC        |
| `public_key_path`   | string | No       | - | Path to the public key used for securing ALS communication when transport-level encryption or authentication is enabled.        |
| `private_key_path`   | string | No       | - | Path to the privare key used for securing ALS communication when transport-level encryption or authentication is enabled.        |
| `max_message_size` | int     | No       | `1000000000` |Maximum size of a single gRPC message that the ALS server is allowed to receive from Envoy.      |
| `max_header_limit` | int     | No       | `8192` | Maximum allowed size of request or response headers processed by the ALS server      |


## Configuration Examples

#### Integrate Moesif Publisher

For Moesif analytics integration, the following publisher-specific attributes must be configured under the `settings` section. These parameters control authentication, batching behavior, and publish intervals for efficient analytics delivery. The required attributes are as follows.

| Parameter              | Type    | Required | Description                               |
| ---------------------- | ------- | -------- | ----------------------------------------- |
| `application_id`       | string  | Yes      | Analytics platform application identifier |
| `publish_interval`     | int     | Yes       | Interval (seconds) between publish cycles |
| `event_queue_size`     | int     | Yes       | Maximum events held in memory             |
| `batch_size`           | int     | Yes       | Maximum events per batch                  |
| `timer_wakeup_seconds` | int     | Yes       | Publisher timer resolution                |


```yaml
analytics:
  enabled: true

  publishers:
    - type: moesif
      enabled: true
      settings:
        application_id: <MOESIF_APPLICATION_ID>
        publish_interval: 5
        event_queue_size: 10000
        batch_size: 50
        timer_wakeup_seconds: 3

  grpc_access_logs:
    host: policy-engine
    log_name: "envoy_access_log"
    buffer_flush_interval: 1000000000
    buffer_size_bytes: 16384
    grpc_request_timeout: 20000000000

  access_logs_service:
    als_server_port: 18090
    shutdown_timeout: 600
    als_plain_text: true
    max_message_size: 1000000000
    max_header_limit: 8192
```


## Use Cases

* **API Usage Visibility** – Understand how APIs are consumed across tenants and applications.
* **Operational Insights** – Observe traffic volume, response behavior, and latency trends.
* **Business Intelligence** – Support product and business decisions using API analytics data.
* **Platform Monitoring** – Gain observability into API behavior without impacting performance.


