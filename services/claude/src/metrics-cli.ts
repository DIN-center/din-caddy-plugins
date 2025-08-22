#!/usr/bin/env node

/**
 * DIN Metrics Query CLI
 * Interactive command-line interface for querying infrastructure metrics
 */

import * as readline from 'readline';
const dotenv = require('dotenv');
import chalk from 'chalk';
import { MetricsClient } from './metrics-client';
import { NLToPromQL } from './nl-to-promql';
import { MetricsInterpreter } from './metrics-interpreter';

// Load environment variables
dotenv.config();

class MetricsQueryCLI {
  private rl: readline.Interface;
  private metricsClient: MetricsClient;
  private parser: NLToPromQL;
  private interpreter: MetricsInterpreter;
  private isRunning: boolean = false;
  
  constructor() {
    const signozApiKey = process.env.SIGNOZ_API_KEY;
    const claudeApiKey = process.env.CLAUDE_API_KEY;
    const signozUrl = process.env.SIGNOZ_BASE_URL || 'https://hzzj-wtre.us.signoz.cloud';
    
    if (!signozApiKey) {
      console.error(chalk.red('Error: SIGNOZ_API_KEY environment variable is required'));
      process.exit(1);
    }
    
    this.metricsClient = new MetricsClient(signozApiKey, signozUrl);
    this.parser = new NLToPromQL();
    this.interpreter = new MetricsInterpreter(claudeApiKey);
    
    this.rl = readline.createInterface({
      input: process.stdin,
      output: process.stdout,
      prompt: chalk.cyan('DIN Metrics> '),
    });
    
    if (!claudeApiKey) {
      console.log(chalk.yellow('Note: CLAUDE_API_KEY not set. Using local analysis only.'));
    }
  }
  
  async start() {
    this.isRunning = true;
    
    console.log(chalk.green('\n================================================='));
    console.log(chalk.green('     DIN Infrastructure Metrics Query System'));
    console.log(chalk.green('================================================='));
    console.log(chalk.gray('\nAsk questions about your infrastructure metrics.'));
    console.log(chalk.gray('Examples:'));
    console.log(chalk.gray('  - Which providers have the most errors?'));
    console.log(chalk.gray('  - Show me latency trends for the last hour'));
    console.log(chalk.gray('  - What is the current health status?'));
    console.log(chalk.gray('  - Show error rate for BSC network'));
    console.log(chalk.gray('\nType "help" for more examples or "exit" to quit.\n'));
    
    this.rl.prompt();
    
    this.rl.on('line', async (input) => {
      const query = input.trim();
      
      if (query.toLowerCase() === 'exit' || query.toLowerCase() === 'quit') {
        this.stop();
        return;
      }
      
      if (query.toLowerCase() === 'help') {
        this.showHelp();
        this.rl.prompt();
        return;
      }
      
      if (query.toLowerCase() === 'clear') {
        console.clear();
        this.rl.prompt();
        return;
      }
      
      if (query) {
        await this.processQuery(query);
      }
      
      this.rl.prompt();
    });
    
    this.rl.on('close', () => {
      this.stop();
    });
  }
  
  async processQuery(userQuery: string) {
    console.log(chalk.gray('\nProcessing query...'));
    
    try {
      // Parse the natural language query into PromQL
      const parsedQuery = this.parser.parseQuery(userQuery);
      
      console.log(chalk.gray(`Query type: ${parsedQuery.description}`));
      console.log(chalk.gray(`Time range: ${new Date(parsedQuery.timeRange.start).toLocaleString()} to ${new Date(parsedQuery.timeRange.end).toLocaleString()}`));
      console.log(chalk.gray(`PromQL: ${parsedQuery.query}`));
      
      // Execute the PromQL query
      const metricResult = await this.metricsClient.executePromQL(
        parsedQuery.query,
        parsedQuery.timeRange
      );
      
      console.log(chalk.gray(`Found ${metricResult.series.length} series\n`));
      
      // Interpret the results
      const interpretation = await this.interpreter.interpret(
        userQuery,
        metricResult,
        parsedQuery.description
      );
      
      // Display results
      this.displayResults(interpretation);
      
    } catch (error: any) {
      console.error(chalk.red('Error processing query:'), error.message);
      if (error.response?.data) {
        console.error(chalk.red('API Error:'), error.response.data);
      }
    }
  }
  
  displayResults(interpretation: any) {
    console.log(chalk.blue('\n' + '='.repeat(60)));
    console.log(chalk.blue.bold('Summary:'));
    console.log(chalk.white(interpretation.summary));
    
    if (interpretation.insights && interpretation.insights.length > 0) {
      console.log(chalk.blue.bold('\nInsights:'));
      for (const insight of interpretation.insights) {
        console.log(chalk.white(`  • ${insight}`));
      }
    }
    
    if (interpretation.recommendations && interpretation.recommendations.length > 0) {
      console.log(chalk.yellow.bold('\nRecommendations:'));
      for (const rec of interpretation.recommendations) {
        console.log(chalk.yellow(`  → ${rec}`));
      }
    }
    
    // Display top items if available
    if (interpretation.details?.topItems && interpretation.details.topItems.length > 0) {
      console.log(chalk.gray.bold('\nTop Items:'));
      const itemsToShow = Math.min(5, interpretation.details.topItems.length);
      for (let i = 0; i < itemsToShow; i++) {
        const item = interpretation.details.topItems[i];
        const label = item.labels.provider || item.labels.method || item.labels.service || 'unknown';
        const value = item.latestValue.toFixed(3);
        const trend = item.trend;
        const trendIcon = trend === 'increasing' ? '↑' : trend === 'decreasing' ? '↓' : '→';
        console.log(chalk.gray(`  ${i + 1}. ${label}: ${value} ${trendIcon}`));
      }
    }
    
    console.log(chalk.blue('='.repeat(60) + '\n'));
  }
  
  showHelp() {
    console.log(chalk.cyan('\n=== Query Examples ===\n'));
    
    console.log(chalk.white.bold('Error Queries:'));
    console.log(chalk.gray('  • Which providers have the most errors?'));
    console.log(chalk.gray('  • Show error trends for the last hour'));
    console.log(chalk.gray('  • What methods are failing the most?'));
    console.log(chalk.gray('  • Error rate for ethereum-mainnet'));
    
    console.log(chalk.white.bold('\nLatency Queries:'));
    console.log(chalk.gray('  • Which providers are slowest?'));
    console.log(chalk.gray('  • Show p99 latency trends'));
    console.log(chalk.gray('  • Latency by method for the last 30 minutes'));
    
    console.log(chalk.white.bold('\nHealth Queries:'));
    console.log(chalk.gray('  • What is the current health status?'));
    console.log(chalk.gray('  • Which providers are unhealthy?'));
    console.log(chalk.gray('  • Show health check results'));
    
    console.log(chalk.white.bold('\nTraffic Queries:'));
    console.log(chalk.gray('  • Show request volume by provider'));
    console.log(chalk.gray('  • What is the overall traffic rate?'));
    console.log(chalk.gray('  • Request rate by method'));
    
    console.log(chalk.white.bold('\nTime Ranges:'));
    console.log(chalk.gray('  • last 5 minutes, last 30 minutes'));
    console.log(chalk.gray('  • last hour, last 3 hours, last 6 hours'));
    console.log(chalk.gray('  • last 24 hours, last week'));
    
    console.log(chalk.white.bold('\nCommands:'));
    console.log(chalk.gray('  • help - Show this help message'));
    console.log(chalk.gray('  • clear - Clear the screen'));
    console.log(chalk.gray('  • exit - Exit the program\n'));
  }
  
  stop() {
    if (!this.isRunning) return;
    
    console.log(chalk.green('\nGoodbye!'));
    this.isRunning = false;
    this.rl.close();
    process.exit(0);
  }
}

// Main execution
if (require.main === module) {
  const cli = new MetricsQueryCLI();
  cli.start().catch((error) => {
    console.error(chalk.red('Fatal error:'), error);
    process.exit(1);
  });
}