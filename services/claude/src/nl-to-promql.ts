/**
 * Natural Language to PromQL Converter
 * Converts user questions into PromQL queries for metrics
 */

export interface ParsedMetricQuery {
  query: string;           // The PromQL query
  description: string;     // Human-readable description
  aggregationType: string; // Type of aggregation (rate, sum, percentile, etc.)
  timeRange: {
    start: number;
    end: number;
    step: number;
  };
}

export class NLToPromQL {
  /**
   * Parse natural language query into PromQL
   */
  parseQuery(userQuery: string): ParsedMetricQuery {
    const query = userQuery.toLowerCase();
    const timeRange = this.extractTimeRange(query);
    
    // Error rate queries
    if (query.includes('error') || query.includes('fail')) {
      if (query.includes('provider')) {
        return {
          query: 'sum(rate(din_http_request_count{health_status!="Healthy"}[5m])) by (provider)',
          description: 'Error rate by provider',
          aggregationType: 'rate',
          timeRange,
        };
      }
      if (query.includes('method')) {
        return {
          query: 'sum(rate(din_http_request_count{health_status!="Healthy"}[5m])) by (method)',
          description: 'Error rate by method',
          aggregationType: 'rate',
          timeRange,
        };
      }
      if (query.includes('network') || query.includes('service')) {
        return {
          query: 'sum(rate(din_http_request_count{health_status!="Healthy"}[5m])) by (service)',
          description: 'Error rate by network/service',
          aggregationType: 'rate',
          timeRange,
        };
      }
      if (query.includes('trend') || query.includes('over time')) {
        return {
          query: 'sum(rate(din_http_request_count{health_status!="Healthy"}[5m]))',
          description: 'Error rate trend over time',
          aggregationType: 'rate',
          timeRange,
        };
      }
      // Default error query
      return {
        query: 'sum(rate(din_http_request_count{health_status!="Healthy"}[5m])) by (provider)',
        description: 'Error rate by provider',
        aggregationType: 'rate',
        timeRange,
      };
    }
    
    // Latency queries
    if (query.includes('latency') || query.includes('slow') || query.includes('performance')) {
      const percentile = this.extractPercentile(query);
      
      if (query.includes('provider')) {
        return {
          query: `histogram_quantile(${percentile}, sum(rate(din_http_request_duration_milliseconds_bucket[5m])) by (le, provider))`,
          description: `P${percentile * 100} latency by provider`,
          aggregationType: 'percentile',
          timeRange,
        };
      }
      if (query.includes('method')) {
        return {
          query: `histogram_quantile(${percentile}, sum(rate(din_http_request_duration_milliseconds_bucket[5m])) by (le, method))`,
          description: `P${percentile * 100} latency by method`,
          aggregationType: 'percentile',
          timeRange,
        };
      }
      // Default latency query
      return {
        query: `histogram_quantile(${percentile}, sum(rate(din_http_request_duration_milliseconds_bucket[5m])) by (le))`,
        description: `Overall P${percentile * 100} latency`,
        aggregationType: 'percentile',
        timeRange,
      };
    }
    
    // Health status queries
    if (query.includes('health') || query.includes('status') || query.includes('unhealthy')) {
      if (query.includes('current') || query.includes('now')) {
        return {
          query: 'sum(increase(din_health_check_count[1m])) by (provider, health_status) > 0',
          description: 'Current health status by provider',
          aggregationType: 'instant',
          timeRange: {
            ...timeRange,
            start: Date.now() - 60000, // Last 1 minute for current status
          },
        };
      }
      return {
        query: 'sum(rate(din_health_check_count[5m])) by (provider, health_status)',
        description: 'Health check results by provider',
        aggregationType: 'rate',
        timeRange,
      };
    }
    
    // Traffic/request rate queries
    if (query.includes('traffic') || query.includes('request') || query.includes('volume')) {
      if (query.includes('provider')) {
        return {
          query: 'sum(rate(din_http_request_count[5m])) by (provider)',
          description: 'Request rate by provider',
          aggregationType: 'rate',
          timeRange,
        };
      }
      if (query.includes('method')) {
        return {
          query: 'sum(rate(din_http_request_count[5m])) by (method)',
          description: 'Request rate by method',
          aggregationType: 'rate',
          timeRange,
        };
      }
      return {
        query: 'sum(rate(din_http_request_count[5m]))',
        description: 'Overall request rate',
        aggregationType: 'rate',
        timeRange,
      };
    }
    
    // Success rate queries
    if (query.includes('success') || query.includes('reliability')) {
      return {
        query: 'sum(rate(din_http_request_count{health_status="Healthy"}[5m])) by (provider) / sum(rate(din_http_request_count[5m])) by (provider)',
        description: 'Success rate by provider',
        aggregationType: 'ratio',
        timeRange,
      };
    }
    
    // Block number queries
    if (query.includes('block') || query.includes('sync') || query.includes('behind')) {
      return {
        query: 'din_health_check_block_number',
        description: 'Latest block number by provider',
        aggregationType: 'gauge',
        timeRange: {
          ...timeRange,
          start: Date.now() - 300000, // Last 5 minutes for block numbers
        },
      };
    }
    
    // Default to error rate by provider
    return {
      query: 'sum(rate(din_http_request_count{health_status!="Healthy"}[5m])) by (provider)',
      description: 'Error rate by provider (default)',
      aggregationType: 'rate',
      timeRange,
    };
  }
  
  /**
   * Extract time range from query
   */
  private extractTimeRange(query: string): { start: number; end: number; step: number } {
    const now = Date.now();
    
    // Check for specific time periods
    if (query.includes('last hour') || query.includes('past hour')) {
      return { start: now - 3600000, end: now, step: 60 };
    }
    if (query.includes('last 3 hour') || query.includes('past 3 hour')) {
      return { start: now - 10800000, end: now, step: 300 };
    }
    if (query.includes('last 6 hour') || query.includes('past 6 hour')) {
      return { start: now - 21600000, end: now, step: 300 };
    }
    if (query.includes('last 12 hour') || query.includes('past 12 hour')) {
      return { start: now - 43200000, end: now, step: 600 };
    }
    if (query.includes('last 24 hour') || query.includes('past 24 hour') || query.includes('last day')) {
      return { start: now - 86400000, end: now, step: 900 };
    }
    if (query.includes('last week') || query.includes('past week')) {
      return { start: now - 604800000, end: now, step: 3600 };
    }
    if (query.includes('last 30 min') || query.includes('past 30 min')) {
      return { start: now - 1800000, end: now, step: 30 };
    }
    if (query.includes('last 15 min') || query.includes('past 15 min')) {
      return { start: now - 900000, end: now, step: 30 };
    }
    if (query.includes('last 5 min') || query.includes('past 5 min')) {
      return { start: now - 300000, end: now, step: 15 };
    }
    
    // Extract numeric values
    const hoursMatch = query.match(/(\d+)\s*hour/);
    if (hoursMatch) {
      const hours = parseInt(hoursMatch[1]);
      return { start: now - (hours * 3600000), end: now, step: hours > 6 ? 600 : 300 };
    }
    
    const minutesMatch = query.match(/(\d+)\s*min/);
    if (minutesMatch) {
      const minutes = parseInt(minutesMatch[1]);
      return { start: now - (minutes * 60000), end: now, step: minutes > 30 ? 60 : 30 };
    }
    
    // Default to last hour
    return { start: now - 3600000, end: now, step: 60 };
  }
  
  /**
   * Extract percentile from query
   */
  private extractPercentile(query: string): number {
    if (query.includes('p99') || query.includes('99th')) return 0.99;
    if (query.includes('p95') || query.includes('95th')) return 0.95;
    if (query.includes('p90') || query.includes('90th')) return 0.90;
    if (query.includes('p50') || query.includes('median')) return 0.50;
    return 0.95; // Default to p95
  }
  
  /**
   * Extract specific provider, method, or service filters
   */
  extractFilters(query: string): Record<string, string> {
    const filters: Record<string, string> = {};
    
    // Extract provider
    const providerMatch = query.match(/provider[:\s]+([a-z0-9\-\.]+)/i);
    if (providerMatch) {
      filters.provider = providerMatch[1];
    }
    
    // Extract method
    const methodMatch = query.match(/(eth_\w+|get\w+|send\w+)/i);
    if (methodMatch) {
      filters.method = methodMatch[1];
    }
    
    // Extract network/service
    const networkMatch = query.match(/(ethereum|bsc|polygon|solana|arbitrum|optimism|avalanche|fantom|harmony|celo)[\-\s]*(mainnet|testnet)?/i);
    if (networkMatch) {
      filters.service = `${networkMatch[1]}${networkMatch[2] ? '-' + networkMatch[2] : ''}`;
    }
    
    return filters;
  }
}