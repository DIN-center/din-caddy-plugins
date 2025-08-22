# DIN Metrics Query System

A natural language interface for querying DIN infrastructure metrics using Prometheus metrics and Claude AI.

## Overview

This system allows you to ask questions about your infrastructure in plain English and get intelligent insights from metrics data. It converts natural language queries to PromQL, fetches metrics from SigNoz, and uses Claude AI to provide actionable analysis.

## Features

- **Natural Language Queries**: Ask questions in plain English
- **Real-time Metrics**: Query Prometheus metrics collected from DIN infrastructure
- **AI-Powered Analysis**: Claude AI provides insights and recommendations
- **Efficient**: Uses pre-aggregated metrics instead of processing raw logs
- **Interactive CLI**: User-friendly command-line interface

## Quick Start

### Prerequisites

1. Node.js 18+ and npm
2. Environment variables:
   ```bash
   SIGNOZ_API_KEY=your_signoz_api_key
   SIGNOZ_BASE_URL=https://your-signoz-instance.com  # Optional, defaults to cloud instance
   CLAUDE_API_KEY=your_anthropic_api_key  # Optional, uses local analysis if not set
   ```

### Installation

```bash
# Install dependencies
npm install

# Build the project
npm run build
```

### Usage

```bash
# Start the interactive CLI
npm run metrics

# Or directly
node dist/metrics-cli.js
```

### Example Queries

```
DIN Metrics> Which providers have the most errors?
DIN Metrics> Show me latency trends for the last hour
DIN Metrics> What is the current health status?
DIN Metrics> Show error rate for BSC network
DIN Metrics> Which methods are failing the most?
```

## Project Structure

```
services/claude/
├── src/
│   ├── metrics-client.ts      # SigNoz API client for PromQL queries
│   ├── nl-to-promql.ts        # Natural language to PromQL converter
│   ├── metrics-interpreter.ts # Claude AI analysis engine
│   └── metrics-cli.ts         # Interactive CLI interface
├── dist/                      # Compiled JavaScript (git-ignored)
├── CLAUDE.md                  # Detailed metrics documentation
├── README.md                  # This file
├── package.json               # Node.js configuration
├── tsconfig.json             # TypeScript configuration
└── .env                      # Environment variables (git-ignored)
```

## Available Metrics

The system queries these Prometheus metrics:

- **din_http_request_count**: Request counts with health status
- **din_http_request_duration_milliseconds**: Request latency histograms
- **din_health_check_count**: Provider health check results
- **din_health_check_block_number**: Latest block numbers per provider

See [CLAUDE.md](./CLAUDE.md) for detailed metrics documentation.

## Development

```bash
# Build TypeScript
npm run build

# Clean build artifacts
npm run clean

# Type checking
npm run type-check

# Development mode with auto-reload
npm run dev
```

## Architecture

1. **User Input**: Natural language question
2. **Query Parser**: Converts to PromQL using pattern matching
3. **Metrics Client**: Executes PromQL via SigNoz API
4. **Data Processing**: Aggregates time series data
5. **AI Analysis**: Claude interprets metrics and patterns
6. **Output**: Formatted insights and recommendations

## Time Ranges

Supported time range expressions:
- Relative: "last 5 minutes", "last hour", "last 24 hours"
- Named: "today", "yesterday", "last week"
- Specific durations: "last 30 minutes", "past 6 hours"

## Troubleshooting

### No metrics data returned
- Verify SigNoz API key is correct
- Check if metrics are being collected for the time range
- Ensure proper network connectivity to SigNoz

### Claude API errors
- Verify CLAUDE_API_KEY is set correctly
- System falls back to local analysis if Claude is unavailable

### High error rates detected
- Review provider health status in the insights
- Check recent deployments or configuration changes
- Consider implementing suggested failover strategies

## License

MIT

## Support

For issues or questions, please contact the DIN infrastructure team.