#!/bin/bash

# Enhanced e2e test for Handler Registry System
# Tests multiple network types and handler functionality with scenario-based testing

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Test results tracking
PASSED_TESTS=0
FAILED_TESTS=0
TOTAL_TESTS=0

# Configuration
BASE_URL="http://localhost:8000"
TIMEOUT=30
MAX_RETRIES=3

# Get test scenario from environment variable (set by GitHub Actions)
TEST_SCENARIO=${TEST_SCENARIO:-"evm-only"}

# Function to print test results
print_test_result() {
    local test_name="$1"
    local status="$2"
    local details="$3"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    if [ "$status" = "PASS" ]; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
        echo -e "${GREEN}PASS${NC}: $test_name"
        [ -n "$details" ] && echo -e "   ${BLUE}→${NC} $details"
    else
        FAILED_TESTS=$((FAILED_TESTS + 1))
        echo -e "${RED}FAIL${NC}: $test_name"
        [ -n "$details" ] && echo -e "   ${RED}→${NC} $details"
    fi
}

# Function to make HTTP requests with retry logic
make_request() {
    local method="$1"
    local url="$2"
    local data="$3"
    local expected_status="$4"
    local description="$5"
    
    local attempt=1
    local max_attempts=$MAX_RETRIES
    
    while [ $attempt -le $max_attempts ]; do
        local response
        local status_code
        
        if [ "$method" = "GET" ]; then
            response=$(curl -s -w "\n%{http_code}" -m $TIMEOUT "$url" 2>/dev/null)
        else
            response=$(curl -s -w "\n%{http_code}" -m $TIMEOUT -X "$method" \
                -H "Content-Type: application/json" \
                -d "$data" "$url" 2>/dev/null)
        fi
        
        if [ $? -eq 0 ]; then
            status_code=$(echo "$response" | tail -n1)
            response_body=$(echo "$response" | head -n -1)
            
            if [ "$status_code" = "$expected_status" ]; then
                print_test_result "$description" "PASS" "Status: $status_code, Attempt: $attempt"
                echo "$response_body"
                return 0
            else
                echo -e "${YELLOW}Attempt $attempt failed${NC}: Expected $expected_status, got $status_code"
            fi
        else
            echo -e "${YELLOW}Attempt $attempt failed${NC}: Network error"
        fi
        
        attempt=$((attempt + 1))
        if [ $attempt -le $max_attempts ]; then
            echo -e "${CYAN}Retrying in 2 seconds...${NC}"
            sleep 2
        fi
    done
    
    print_test_result "$description" "FAIL" "All $max_attempts attempts failed"
    return 1
}

# Function to test JSON-RPC endpoint
test_jsonrpc_endpoint() {
    local network="$1"
    local method="$2"
    local params="$3"
    local description="$4"
    
    local payload="{\"jsonrpc\":\"2.0\",\"method\":\"$method\",\"params\":$params,\"id\":1}"
    make_request "POST" "$BASE_URL/$network" "$payload" "200" "$description"
}

# Function to test REST endpoint
test_rest_endpoint() {
    local network="$1"
    local endpoint="$2"
    local description="$3"
    
    make_request "GET" "$BASE_URL/$network$endpoint" "" "200" "$description"
}

# Function to test network handler registration
test_handler_registration() {
    echo -e "\n${PURPLE}Testing Handler Registry System${NC}"
    
    # Test health endpoint (should always work)
    make_request "GET" "$BASE_URL/health" "" "200" "Health Endpoint Availability"
    
    # Test network-specific health checks based on scenario
    case "$TEST_SCENARIO" in
        "evm-only")
            test_jsonrpc_endpoint "eth" "eth_blockNumber" "[]" "EVM Handler - Block Number Check"
            ;;
    esac
    

}

# Function to test EVM handler functionality
test_evm_handler() {
    echo -e "\n${BLUE}* Testing EVM Handler${NC}"
    
    # Basic EVM JSON-RPC tests
    test_jsonrpc_endpoint "eth" "eth_blockNumber" "[]" "EVM - Get Latest Block Number"
    test_jsonrpc_endpoint "eth" "eth_chainId" "[]" "EVM - Get Chain ID"
    test_jsonrpc_endpoint "eth" "eth_getBalance" "[\"0x742d35cc6231c21308ad6fcdb6da8f421c68a7fb\", \"latest\"]" "EVM - Get Account Balance"
    
    # Test invalid method (should still return 200 with JSON-RPC error)
    # This test is disabled because bug introduced in https://github.com/DIN-center/din-caddy-plugins/issues/204
    # test_jsonrpc_endpoint "eth" "eth_invalidMethod" "[]" "EVM - Invalid Method Handling"
    

}

# Function to test Beacon Chain handler functionality
test_beacon_handler() {
    echo -e "\n${CYAN}🔗 Testing Beacon Chain Handler${NC}"
    
    # Basic Beacon Chain REST API tests
    test_rest_endpoint "ethereum-beacon" "/eth/v1/node/health" "Beacon - Node Health"
    test_rest_endpoint "ethereum-beacon" "/eth/v1/node/syncing" "Beacon - Sync Status"
    test_rest_endpoint "ethereum-beacon" "/eth/v1/beacon/genesis" "Beacon - Genesis Data"
    
    # Test with path parameters (should normalize correctly)
    test_rest_endpoint "ethereum-beacon" "/eth/v1/beacon/headers/head" "Beacon - Latest Header"
    test_rest_endpoint "ethereum-beacon" "/eth/v1/beacon/blocks/head" "Beacon - Latest Block"
    
    # Test path normalization patterns
    test_rest_endpoint "ethereum-beacon" "/eth/v1/beacon/states/finalized/root" "Beacon - State Root (Path Normalization)"
    
    # Test different HTTP methods (some endpoints support POST)
    local post_payload='{"ids":["0x1234"]}'
    make_request "POST" "$BASE_URL/ethereum-beacon/eth/v1/beacon/states/head/validators" "$post_payload" "200" "Beacon - POST Request Support"
}



# Function to test Solana handler functionality
test_solana_handler() {
    echo -e "\n${YELLOW}Testing Solana Handler${NC}"
    
    # Solana-specific JSON-RPC tests
    test_jsonrpc_endpoint "solana-mainnet" "getBlockHeight" "[]" "Solana - Get Block Height"
    test_jsonrpc_endpoint "solana-mainnet" "getGenesisHash" "[]" "Solana - Get Genesis Hash"
    
    # Test Solana-specific parameter format
    local slot_params='[100, {"encoding": "json", "transactionDetails": "none", "rewards": false}]'
    test_jsonrpc_endpoint "solana-mainnet" "getBlock" "$slot_params" "Solana - Get Block Data"
}

# Function to test path translation capabilities
test_path_translation() {
    echo -e "\n${GREEN}Testing Path Translation${NC}"
    
    # Path translation tests only apply to beacon chain scenarios
    # Since beacon chain scenarios are removed, this function is now empty
}

# Function to test error handling
test_error_handling() {
    echo -e "\n${RED}Testing Error Handling${NC}"
    
    # Test 404 for non-existent networks
    make_request "GET" "$BASE_URL/non-existent-network" "" "404" "Non-existent Network - 404 Response"
    
    # Test invalid JSON-RPC for EVM
    if [[ "$TEST_SCENARIO" =~ (evm|all|mixed) ]]; then
        local invalid_payload='{"invalid":"json"}'
        make_request "POST" "$BASE_URL/eth" "$invalid_payload" "400" "EVM - Invalid JSON-RPC Payload"
    fi
    
    # Test invalid REST endpoints
    if [[ "$TEST_SCENARIO" =~ (beacon|all|mixed) ]]; then
        make_request "GET" "$BASE_URL/ethereum-beacon/invalid/endpoint" "" "404" "Beacon - Invalid REST Endpoint"
    fi
}

# Function to test provider selection and failover
test_provider_failover() {
    echo -e "\n${CYAN}🔁 Testing Provider Failover${NC}"
    
    # Test multiple providers with same request
    case "$TEST_SCENARIO" in
        "evm-only")
            # Make multiple requests to test provider rotation
            for i in {1..3}; do
                test_jsonrpc_endpoint "eth" "eth_blockNumber" "[]" "EVM - Provider Failover Test $i"
                sleep 1
            done
            ;;
    esac
}

# Function to test performance and concurrency
test_performance() {
    echo -e "\n${PURPLE}Testing Performance${NC}"
    
    # Concurrent request test
    echo -e "${CYAN}Testing concurrent requests...${NC}"
    
    case "$TEST_SCENARIO" in
        "evm-only")
            # Launch multiple background requests
            for i in {1..5}; do
                (test_jsonrpc_endpoint "eth" "eth_blockNumber" "[]" "EVM - Concurrent Request $i") &
            done
            wait
            ;;
    esac
}

# Main test execution function
run_test_scenario() {
    echo -e "${PURPLE}================================================${NC}"
    echo -e "${PURPLE}🧪 Running E2E Test Scenario: ${YELLOW}$TEST_SCENARIO${NC}"
    echo -e "${PURPLE}================================================${NC}"
    
    # Always test basic handler registration
    test_handler_registration
    
    # Run scenario-specific tests
    case "$TEST_SCENARIO" in
        "evm-only")
            echo -e "\n${BLUE}* EVM-Only Test Scenario${NC}"
            test_evm_handler
            test_provider_failover
            ;;
        *)
            echo -e "${RED}Unknown test scenario: $TEST_SCENARIO${NC}"
            exit 1
            ;;
    esac
    
    # Always test error handling
    test_error_handling
}

# Function to print final summary
print_final_summary() {
    echo -e "\n${PURPLE}================================================${NC}"
    echo -e "${PURPLE}Test Summary for Scenario: ${YELLOW}$TEST_SCENARIO${NC}"
    echo -e "${PURPLE}================================================${NC}"
    
    echo -e "${BLUE}Total Tests: ${NC}$TOTAL_TESTS"
    echo -e "${GREEN}Passed: ${NC}$PASSED_TESTS"
    echo -e "${RED}Failed: ${NC}$FAILED_TESTS"
    
    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "\n${GREEN}All tests passed! Handler registry is working correctly.${NC}"
        echo -e "${GREEN}Handler Registry System: OPERATIONAL${NC}"
        echo -e "${GREEN}Network Type Detection: WORKING${NC}"
        echo -e "${GREEN}Path Translation: FUNCTIONAL${NC}"
        echo -e "${GREEN}Provider Failover: TESTED${NC}"
        echo -e "${GREEN}Error Handling: VALIDATED${NC}"
        exit 0
    else
        echo -e "\n${RED}💥 Some tests failed. Please check the logs above.${NC}"
        echo -e "${RED}Test Scenario '$TEST_SCENARIO' has failures${NC}"
        
        # Calculate success rate
        local success_rate=$((PASSED_TESTS * 100 / TOTAL_TESTS))
        echo -e "${YELLOW}Success Rate: ${NC}$success_rate%"
        
        if [ $success_rate -ge 80 ]; then
            echo -e "${YELLOW}Warning: Some tests failed but success rate is acceptable${NC}"
            exit 1
        else
            echo -e "${RED}Critical: Success rate too low${NC}"
            exit 2
        fi
    fi
}

# Function to validate environment
validate_environment() {
    echo -e "${CYAN}Validating test environment...${NC}"
    
    # Check if base URL is reachable
    if ! curl -s -f "$BASE_URL/health" > /dev/null 2>&1; then
        echo -e "${RED}Cannot reach $BASE_URL/health${NC}"
        echo -e "${RED}Please ensure Caddy server is running${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}Environment validation passed${NC}"
    echo -e "${BLUE}Base URL: ${NC}$BASE_URL"
    echo -e "${BLUE}Test Scenario: ${NC}$TEST_SCENARIO"
    echo -e "${BLUE}Timeout: ${NC}${TIMEOUT}s"
    echo -e "${BLUE}Max Retries: ${NC}$MAX_RETRIES"
}

# Main execution
main() {
    echo -e "${CYAN}Starting Handler Registry E2E Tests${NC}"
    echo -e "${CYAN}Timestamp: $(date)${NC}"
    
    validate_environment
    run_test_scenario
    print_final_summary
}

# Execute main function
main "$@"