#!/bin/bash

# Wait for the server to be ready
max_attempts=30
attempt=1
echo "Waiting for server to be ready..."
while ! curl -s localhost:8000/health > /dev/null; do
    if [ $attempt -eq $max_attempts ]; then
        echo "Server failed to become ready after $max_attempts attempts"
        exit 1
    fi
    echo "Attempt $attempt: Server not ready yet, waiting..."
    sleep 2
    ((attempt++))
done

echo "Server is ready. Waiting 10 seconds before starting requests..."
sleep 10
echo "Starting requests..."

for i in {1..10}; do
    # Make the POST request and capture both the response and HTTP status code
    response_data=$(curl -s -w "\n%{http_code}" localhost:8000/eth \
        --data '{"id": 0, "jsonrpc": "2.0", "method": "eth_blockNumber"}' \
        -H 'content-type: application/json')
    
    # Extract the status code from the last line
    status_code=$(echo "$response_data" | tail -n1)
    # Extract the response body (everything except the last line)
    response_body=$(echo "$response_data" | sed \$d)
    
    # Check if the response is 200
    if [ "$status_code" -ne 200 ]; then
        echo "Request $i failed with response code $status_code"
        echo "Response body: $response_body"
        exit 1
    else
        echo "Request $i succeeded with response code $status_code"
        echo "Response body: $response_body"
    fi
    
    # Add a small delay between requests
    sleep 0.5
done