/**
 * Claude Metrics Interpreter
 * Uses Claude AI to analyze and interpret time series metrics data
 */

import Anthropic from '@anthropic-ai/sdk';
import { MetricResult, MetricSeries } from './metrics-client';

export interface InterpretationResult {
  summary: string;
  insights: string[];
  recommendations: string[];
  details?: any;
}

export class MetricsInterpreter {
  private claude: Anthropic | null = null;
  
  constructor(apiKey?: string) {
    if (apiKey) {
      this.claude = new Anthropic({
        apiKey: apiKey,
      });
    }
  }
  
  /**
   * Interpret metric results using Claude
   */
  async interpret(
    userQuery: string,
    metricResult: MetricResult,
    queryDescription: string
  ): Promise<InterpretationResult> {
    // If no Claude API key, use local analysis
    if (!this.claude) {
      return this.localInterpretation(userQuery, metricResult, queryDescription);
    }
    
    try {
      // Prepare data for Claude
      const dataForClaude = this.prepareDataForClaude(metricResult);
      
      const prompt = `You are analyzing metrics data from a distributed infrastructure network (DIN) that manages 30+ blockchain networks and 40+ RPC providers.

User Question: ${userQuery}
Metric Query: ${queryDescription}

Metrics Data:
${JSON.stringify(dataForClaude, null, 2)}

Please analyze this metrics data and provide:
1. A clear, concise summary answering the user's question (2-3 sentences)
2. 3-5 key insights about the data (patterns, anomalies, trends)
3. 3-4 actionable recommendations based on the findings

Format your response as JSON with this structure:
{
  "summary": "...",
  "insights": ["insight1", "insight2", ...],
  "recommendations": ["recommendation1", "recommendation2", ...]
}`;

      const response = await this.claude.messages.create({
        model: 'claude-sonnet-4-20250514',
        max_tokens: 1000,
        messages: [
          {
            role: 'user',
            content: prompt,
          },
        ],
      });
      
      // Parse Claude's response
      const content = response.content[0];
      if (content.type === 'text') {
        try {
          // Try to extract JSON from the response (might be wrapped in markdown)
          let jsonText = content.text;
          
          // Check if JSON is wrapped in markdown code block
          const jsonMatch = content.text.match(/```(?:json)?\s*(\{[\s\S]*\})\s*```/);
          if (jsonMatch) {
            jsonText = jsonMatch[1];
          }
          
          const parsed = JSON.parse(jsonText);
          return {
            summary: parsed.summary || 'No summary available',
            insights: parsed.insights || [],
            recommendations: parsed.recommendations || [],
            details: dataForClaude,
          };
        } catch (parseError) {
          // If JSON parsing fails, return the text as summary
          console.error('Failed to parse Claude response as JSON:', parseError);
          return {
            summary: content.text,
            insights: [],
            recommendations: [],
            details: dataForClaude,
          };
        }
      }
    } catch (error) {
      console.error('Claude interpretation failed:', error);
      // Fall back to local interpretation
      return this.localInterpretation(userQuery, metricResult, queryDescription);
    }
    
    // Fallback
    return this.localInterpretation(userQuery, metricResult, queryDescription);
  }
  
  /**
   * Prepare metrics data for Claude analysis
   */
  private prepareDataForClaude(metricResult: MetricResult): any {
    const summary: any = {
      seriesCount: metricResult.series.length,
      timeRange: this.getTimeRange(metricResult),
      topItems: [],
      aggregatedValues: {},
    };
    
    // Process each series
    for (const series of metricResult.series) {
      const latestValue = this.getLatestValue(series);
      const avgValue = this.getAverageValue(series);
      const trend = this.calculateTrend(series);
      
      const item = {
        labels: series.labels,
        latestValue,
        averageValue: avgValue,
        trend,
        dataPoints: series.values.length,
      };
      
      summary.topItems.push(item);
      
      // Aggregate by key labels
      for (const [key, value] of Object.entries(series.labels)) {
        if (key !== '__name__' && value) {
          if (!summary.aggregatedValues[key]) {
            summary.aggregatedValues[key] = {};
          }
          if (!summary.aggregatedValues[key][value]) {
            summary.aggregatedValues[key][value] = 0;
          }
          summary.aggregatedValues[key][value] += latestValue;
        }
      }
    }
    
    // Sort top items by latest value
    summary.topItems.sort((a: any, b: any) => b.latestValue - a.latestValue);
    
    // Keep only top 20 items for Claude
    if (summary.topItems.length > 20) {
      summary.topItems = summary.topItems.slice(0, 20);
    }
    
    return summary;
  }
  
  /**
   * Local interpretation without Claude
   */
  private localInterpretation(
    userQuery: string,
    metricResult: MetricResult,
    queryDescription: string
  ): InterpretationResult {
    const summary = this.generateLocalSummary(metricResult, queryDescription);
    const insights = this.generateLocalInsights(metricResult);
    const recommendations = this.generateLocalRecommendations(metricResult, userQuery);
    
    return {
      summary,
      insights,
      recommendations,
      details: this.prepareDataForClaude(metricResult),
    };
  }
  
  /**
   * Generate local summary
   */
  private generateLocalSummary(metricResult: MetricResult, queryDescription: string): string {
    if (metricResult.series.length === 0) {
      return 'No data available for the specified query and time range.';
    }
    
    // Find top items
    const topItems = metricResult.series
      .map(s => ({
        label: s.labels.provider || s.labels.method || s.labels.service || 'unknown',
        value: this.getLatestValue(s),
      }))
      .sort((a, b) => b.value - a.value)
      .slice(0, 3);
    
    if (topItems.length === 0) {
      return `Found ${metricResult.series.length} series for ${queryDescription}.`;
    }
    
    const topList = topItems
      .map(item => `${item.label} (${this.formatValue(item.value)})`)
      .join(', ');
    
    return `Top results for ${queryDescription}: ${topList}`;
  }
  
  /**
   * Generate local insights
   */
  private generateLocalInsights(metricResult: MetricResult): string[] {
    const insights: string[] = [];
    
    if (metricResult.series.length === 0) {
      return ['No data available for analysis'];
    }
    
    // Insight about data volume
    insights.push(`Analyzing ${metricResult.series.length} distinct series`);
    
    // Insight about top performer
    const topSeries = metricResult.series
      .sort((a, b) => this.getLatestValue(b) - this.getLatestValue(a))[0];
    if (topSeries) {
      const label = topSeries.labels.provider || topSeries.labels.method || 'item';
      insights.push(`Highest value: ${label} with ${this.formatValue(this.getLatestValue(topSeries))}`);
    }
    
    // Insight about trends
    const increasing = metricResult.series.filter(s => this.calculateTrend(s) === 'increasing').length;
    const decreasing = metricResult.series.filter(s => this.calculateTrend(s) === 'decreasing').length;
    if (increasing > decreasing) {
      insights.push(`${increasing} series showing increasing trend`);
    } else if (decreasing > increasing) {
      insights.push(`${decreasing} series showing decreasing trend`);
    }
    
    return insights;
  }
  
  /**
   * Generate local recommendations
   */
  private generateLocalRecommendations(metricResult: MetricResult, userQuery: string): string[] {
    const recommendations: string[] = [];
    
    if (metricResult.series.length === 0) {
      return ['Check if metrics are being collected for this time range'];
    }
    
    // Recommendations based on query type
    if (userQuery.toLowerCase().includes('error')) {
      const highErrorSeries = metricResult.series
        .filter(s => this.getLatestValue(s) > 0.1)
        .sort((a, b) => this.getLatestValue(b) - this.getLatestValue(a));
      
      if (highErrorSeries.length > 0) {
        const provider = highErrorSeries[0].labels.provider || 'top provider';
        recommendations.push(`Investigate high error rate on ${provider}`);
        recommendations.push('Consider implementing automatic failover for high-error providers');
      }
    }
    
    if (userQuery.toLowerCase().includes('latency')) {
      recommendations.push('Monitor providers with latency above SLA thresholds');
      recommendations.push('Consider load balancing to reduce latency');
    }
    
    if (userQuery.toLowerCase().includes('health')) {
      recommendations.push('Set up alerts for unhealthy providers');
      recommendations.push('Review provider redundancy for critical services');
    }
    
    return recommendations;
  }
  
  /**
   * Helper: Get latest value from series
   */
  private getLatestValue(series: MetricSeries): number {
    if (series.values.length === 0) return 0;
    return parseFloat(series.values[series.values.length - 1].value) || 0;
  }
  
  /**
   * Helper: Get average value from series
   */
  private getAverageValue(series: MetricSeries): number {
    if (series.values.length === 0) return 0;
    const sum = series.values.reduce((acc, v) => acc + (parseFloat(v.value) || 0), 0);
    return sum / series.values.length;
  }
  
  /**
   * Helper: Calculate trend
   */
  private calculateTrend(series: MetricSeries): 'increasing' | 'decreasing' | 'stable' {
    if (series.values.length < 2) return 'stable';
    
    const firstHalf = series.values.slice(0, Math.floor(series.values.length / 2));
    const secondHalf = series.values.slice(Math.floor(series.values.length / 2));
    
    const firstAvg = firstHalf.reduce((acc, v) => acc + parseFloat(v.value), 0) / firstHalf.length;
    const secondAvg = secondHalf.reduce((acc, v) => acc + parseFloat(v.value), 0) / secondHalf.length;
    
    const change = ((secondAvg - firstAvg) / firstAvg) * 100;
    
    if (change > 10) return 'increasing';
    if (change < -10) return 'decreasing';
    return 'stable';
  }
  
  /**
   * Helper: Get time range
   */
  private getTimeRange(metricResult: MetricResult): { start?: string; end?: string } {
    if (metricResult.series.length === 0 || metricResult.series[0].values.length === 0) {
      return {};
    }
    
    const firstSeries = metricResult.series[0];
    return {
      start: new Date(firstSeries.values[0].timestamp).toISOString(),
      end: new Date(firstSeries.values[firstSeries.values.length - 1].timestamp).toISOString(),
    };
  }
  
  /**
   * Helper: Format value for display
   */
  private formatValue(value: number): string {
    if (value < 0.01) return value.toExponential(2);
    if (value < 1) return value.toFixed(3);
    if (value < 100) return value.toFixed(2);
    if (value < 1000) return value.toFixed(1);
    return Math.round(value).toLocaleString();
  }
}