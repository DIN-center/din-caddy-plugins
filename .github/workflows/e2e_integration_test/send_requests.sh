#!/bin/bash

for i in {1..10}; do
    # Make the POST request and capture the HTTP status code
    response=$(curl -s -o /dev/null -w "%{http_code}" localhost:8000/eth --data '{"id": 0, "jsonrpc": "2.0", "method": "eth_blockNumber"}' -H 'content-type: application/json')
    
    # Check if the response is 200
    if [ "$response" -ne 200 ]; then
        echo "Request $i failed with response code $response"
        exit 1
    else
        echo "Request $i succeeded with response code $response"
    fi
done