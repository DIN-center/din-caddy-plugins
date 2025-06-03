#!/bin/bash

# Update dashboard UIDs and import
cat /etc/grafana/dashboards/caddy-dashboard.json | \
  sed 's/\${DS_PROMETHEUS}/Prometheus/g' | \
  sed 's/\${DS_LOKI}/Loki/g' | \
  curl -s -u admin:admin -X POST -H "Content-Type: application/json" -d @- \
  http://localhost:3000/api/dashboards/db

cat /etc/grafana/dashboards/din-overview-dashboard.json | \
  sed 's/\${DS_PROMETHEUS}/Prometheus/g' | \
  sed 's/\${DS_LOKI}/Loki/g' | \
  curl -s -u admin:admin -X POST -H "Content-Type: application/json" -d @- \
  http://localhost:3000/api/dashboards/db

echo "Dashboards imported" 