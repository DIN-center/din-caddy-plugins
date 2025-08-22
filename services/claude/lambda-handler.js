/**
 * Simple Lambda handler for DIN Metrics Query
 * No state, no caching, just direct query processing
 */

const { MetricsClient } = require('./dist/metrics-client');
const { NLToPromQL } = require('./dist/nl-to-promql');
const { MetricsInterpreter } = require('./dist/metrics-interpreter');

exports.handler = async (event) => {
    // Get query from URL parameters or body
    let query;
    
    if (event.queryStringParameters?.q) {
        // GET request: /query?q=Which providers have errors
        query = event.queryStringParameters.q;
    } else if (event.body) {
        // POST request with body
        try {
            const body = JSON.parse(event.body);
            query = body.query || body.q;
        } catch {
            query = event.body; // Plain text body
        }
    } else {
        return {
            statusCode: 400,
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                error: 'No query provided. Use ?q=your+query or POST body'
            })
        };
    }
    
    // Initialize components
    const metricsClient = new MetricsClient(
        process.env.SIGNOZ_API_KEY,
        process.env.SIGNOZ_BASE_URL || 'https://hzzj-wtre.us.signoz.cloud'
    );
    const parser = new NLToPromQL();
    const interpreter = new MetricsInterpreter(process.env.CLAUDE_API_KEY);
    
    try {
        // Parse natural language to PromQL
        const parsed = parser.parseQuery(query);
        
        // Execute PromQL query
        const metrics = await metricsClient.executePromQL(
            parsed.query,
            parsed.timeRange
        );
        
        // Get Claude's interpretation
        const interpretation = await interpreter.interpret(
            query,
            metrics,
            parsed.description
        );
        
        // Return simple, readable response
        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                query: query,
                answer: interpretation.summary,
                insights: interpretation.insights,
                recommendations: interpretation.recommendations,
                details: {
                    promql: parsed.query,
                    timeRange: {
                        start: new Date(parsed.timeRange.start).toISOString(),
                        end: new Date(parsed.timeRange.end).toISOString()
                    },
                    seriesCount: metrics.series.length
                }
            }, null, 2)
        };
        
    } catch (error) {
        console.error('Query processing error:', error);
        
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                error: 'Failed to process query',
                message: error.message,
                query: query
            }, null, 2)
        };
    }
};