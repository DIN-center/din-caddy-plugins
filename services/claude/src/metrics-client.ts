/**
 * Metrics Query Client for SigNoz
 * Executes PromQL queries via the SigNoz API
 */

import axios, { AxiosInstance } from 'axios';

export interface TimeRange {
  start: number; // milliseconds
  end: number;   // milliseconds
  step?: number; // seconds (default: 60)
}

export interface MetricSeries {
  labels: Record<string, string>;
  values: Array<{
    timestamp: number;
    value: string;
  }>;
}

export interface MetricResult {
  queryName: string;
  series: MetricSeries[];
}

export class MetricsClient {
  private client: AxiosInstance;
  
  constructor(apiKey: string, baseUrl: string = 'https://hzzj-wtre.us.signoz.cloud') {
    this.client = axios.create({
      baseURL: baseUrl,
      headers: {
        'SIGNOZ-API-KEY': apiKey,
        'Content-Type': 'application/json',
      },
      timeout: 30000,
    });
  }
  
  /**
   * Execute a PromQL query
   */
  async executePromQL(query: string, timeRange: TimeRange): Promise<MetricResult> {
    try {
      const requestBody = {
        start: timeRange.start,
        end: timeRange.end,
        step: timeRange.step || 60,
        compositeQuery: {
          promQueries: {
            A: {
              query: query,
              disabled: false,
            },
          },
          panelType: 'graph',
          queryType: 'promql',
        },
        formatForWeb: true,
      };
      
      const response = await this.client.post('/api/v4/query_range', requestBody);
      
      if (response.data?.status === 'error') {
        throw new Error(`SigNoz API error: ${response.data.error}`);
      }
      
      // Parse the response
      const result = response.data?.data?.result?.[0];
      if (!result) {
        return {
          queryName: 'A',
          series: [],
        };
      }
      
      return {
        queryName: result.queryName || 'A',
        series: result.series || [],
      };
    } catch (error: any) {
      console.error('PromQL query failed:', error.response?.data || error.message);
      throw error;
    }
  }
  
  /**
   * Get error rate by provider
   */
  async getErrorRateByProvider(timeRange: TimeRange): Promise<MetricResult> {
    const query = 'sum(rate(din_http_request_count{health_status!="Healthy"}[5m])) by (provider)';
    return this.executePromQL(query, timeRange);
  }
  
  /**
   * Get latency percentiles
   */
  async getLatencyPercentile(percentile: number, timeRange: TimeRange, filters?: string): Promise<MetricResult> {
    const filterStr = filters ? `{${filters}}` : '';
    const query = `histogram_quantile(${percentile}, sum(rate(din_http_request_duration_milliseconds_bucket${filterStr}[5m])) by (le, provider))`;
    return this.executePromQL(query, timeRange);
  }
  
  /**
   * Get health status distribution
   */
  async getHealthStatus(timeRange: TimeRange): Promise<MetricResult> {
    const query = 'sum(din_health_check_count) by (provider, health_status)';
    return this.executePromQL(query, timeRange);
  }
  
  /**
   * Get request rate by method
   */
  async getRequestRateByMethod(timeRange: TimeRange, provider?: string): Promise<MetricResult> {
    const filter = provider ? `{provider="${provider}"}` : '';
    const query = `sum(rate(din_http_request_count${filter}[5m])) by (method)`;
    return this.executePromQL(query, timeRange);
  }
  
  /**
   * Get total error count over time
   */
  async getErrorTrend(timeRange: TimeRange, provider?: string): Promise<MetricResult> {
    const filter = provider ? `,provider="${provider}"` : '';
    const query = `sum(rate(din_http_request_count{health_status!="Healthy"${filter}}[5m]))`;
    return this.executePromQL(query, timeRange);
  }
}