# Setting Up Local Monitoring with Grafana, Loki, and Prometheus

This guide explains how to set up a complete monitoring stack for DIN Caddy Plugins using Grafana, Loki, and Prometheus.

## Overview

The monitoring stack consists of:

- **Prometheus**: Time-series database for metrics collection
- **Loki**: Log aggregation system
- **Grafana**: Visualization platform for metrics and logs
- **OpenTelemetry Collector**: Collects and forwards telemetry data

## Prerequisites

- Docker and Docker Compose installed
- DIN Caddy Plugins repository cloned

## Quick Start

All components are preconfigured in the `compose.yml` file. To start the entire stack:

```bash
docker-compose up -d
```

## Access the Dashboards

Grafana is accessible at http://localhost:3000 with the following credentials:
- Username: `admin`
- Password: `admin`

## Components and Architecture

### Caddy

Caddy is configured to send telemetry data to the OpenTelemetry collector:
- Metrics exposed on port 2019
- Logs written to caddy.log, which is collected by the OpenTelemetry collector

### OpenTelemetry Collector

The collector:
- Scrapes Prometheus metrics from Caddy
- Reads log files from Caddy
- Exports data to Prometheus and Loki

### Prometheus

Prometheus:
- Scrapes metrics from the OpenTelemetry collector
- Stores time-series data
- Accessible on port 9090

### Loki

Loki:
- Receives logs from the OpenTelemetry collector
- Indexes and stores logs
- Accessible on port 3100

### Grafana

Grafana:
- Configured with Prometheus and Loki as data sources
- Includes pre-built dashboards for DIN monitoring
- Accessible on port 3000

## Directory Structure

```
services/
  ├── grafana/
  │   ├── dashboards/
  │   │   ├── din-health-check-dashboard.json
  │   │   ├── din-overview-dashboard.json
  │   │   └── provider.yml
  │   └── datasources/
  │       ├── loki.yml
  │       └── prometheus.yml
  ├── loki/
  │   └── loki-config.yml
  ├── otel-collector/
  │   └── config.yml
  └── prometheus/
      └── prometheus.yml
```

## Available Dashboards

The monitoring stack comes with pre-configured dashboards:

1. **DIN Overview Dashboard**: Shows general request metrics, error rates, and performance data.
2. **DIN Health Check Dashboard**: Displays detailed health check status for all networks and providers.

## Data Persistence

The following Docker volumes ensure data persistence:

- `prometheus_data`: Stores Prometheus time-series data
- `loki_data`: Stores Loki logs
- `grafana_data`: Stores Grafana configurations and dashboards

## Customization

### Adding Custom Dashboards

You can create and save dashboards directly in Grafana UI. For automatic provisioning:

1. Export the dashboard JSON from Grafana
2. Save it to `services/grafana/dashboards/`
3. Update `services/grafana/dashboards/provider.yml` if needed
4. Restart the Grafana container

### Configuring Metrics Retention

Edit `services/prometheus/prometheus.yml` to adjust the scrape interval and evaluation interval.

For Loki log retention, edit the `retention_period` settings in `services/loki/loki-config.yml`.

## Troubleshooting

### Check Service Status

```bash
docker-compose ps
```

### View Service Logs

```bash
docker-compose logs -f [service_name]
```

Where `[service_name]` can be `grafana`, `loki`, `prometheus`, or `otel-collector`.

### Common Issues

1. **Data not showing in Grafana**: 
   - Check that data sources are correctly configured
   - Verify that the OpenTelemetry collector is running
   - Check for errors in the OpenTelemetry collector logs

2. **Missing logs in Loki**:
   - Ensure caddy.log is being written to the mounted volume
   - Check the OpenTelemetry collector configuration for log collection
   - Verify Loki is running and accessible

3. **No metrics in Prometheus**:
   - Check that Caddy is exposing metrics on port 2019
   - Verify that the OpenTelemetry collector is scraping Caddy metrics
   - Check Prometheus targets status at http://localhost:9090/targets

## Extending the Monitoring Stack

The monitoring stack can be extended with additional components like AlertManager for alerts or additional exporters for the OpenTelemetry collector.

To add custom metrics or logs, update the appropriate configuration files and restart the services. 