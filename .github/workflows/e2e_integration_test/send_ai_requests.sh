#!/bin/bash

# E2E tests for AI Smart Routing module
# Tests live API providers through the din_ai middleware

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# Test results tracking
PASSED_TESTS=0
FAILED_TESTS=0
TOTAL_TESTS=0

# Configuration
BASE_URL="http://localhost:8000"
TIMEOUT=60

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

# Make a non-streaming chat completion request
# Returns: response body on stdout, sets LAST_STATUS_CODE and LAST_HEADERS
make_chat_request() {
    local tier="$1"
    local session_id="$2"
    local stream="$3"

    local headers=(-H "Content-Type: application/json")
    [ -n "$tier" ] && headers+=(-H "X-DIN-Tier: $tier")
    [ -n "$session_id" ] && headers+=(-H "X-DIN-Session-Id: $session_id")

    local payload
    payload=$(cat <<EOF
{
  "model": "ignored",
  "messages": [{"role": "user", "content": "Say exactly: hello world"}],
  "stream": $stream,
  "max_tokens": 20
}
EOF
)

    local tmp_headers
    tmp_headers=$(mktemp)

    local response
    response=$(curl -s -w "\n%{http_code}" -m "$TIMEOUT" \
        -D "$tmp_headers" \
        "${headers[@]}" \
        -d "$payload" \
        "$BASE_URL/v1/chat/completions" 2>/dev/null)

    LAST_STATUS_CODE=$(echo "$response" | tail -n1)
    LAST_BODY=$(echo "$response" | head -n -1)
    LAST_HEADERS=$(cat "$tmp_headers")
    rm -f "$tmp_headers"
}

# Extract a response header value
get_header() {
    local name="$1"
    echo "$LAST_HEADERS" | grep -i "^$name:" | sed 's/^[^:]*: //' | tr -d '\r'
}

# ============================================================
# Test: Health endpoint
# ============================================================
test_health() {
    echo -e "\n${PURPLE}Testing Health Endpoint${NC}"

    local status
    status=$(curl -s -o /dev/null -w "%{http_code}" -m 10 "$BASE_URL/health" 2>/dev/null)

    if [ "$status" = "200" ]; then
        print_test_result "Health endpoint" "PASS" "Status: $status"
    else
        print_test_result "Health endpoint" "FAIL" "Expected 200, got $status"
    fi
}

# ============================================================
# Test: Non-streaming completions
# ============================================================
test_non_streaming() {
    echo -e "\n${BLUE}Testing Non-Streaming Completions${NC}"

    # Fast tier (OpenAI or Anthropic)
    make_chat_request "fast" "" "false"
    if [ "$LAST_STATUS_CODE" = "200" ]; then
        local provider
        provider=$(get_header "X-DIN-Provider")
        local model
        model=$(get_header "X-DIN-Model")
        local tier
        tier=$(get_header "X-DIN-Tier")
        print_test_result "Non-streaming - fast tier" "PASS" "Provider: $provider, Model: $model, Tier: $tier"
    else
        print_test_result "Non-streaming - fast tier" "FAIL" "Status: $LAST_STATUS_CODE, Body: $LAST_BODY"
    fi

    # Balanced tier
    make_chat_request "balanced" "" "false"
    if [ "$LAST_STATUS_CODE" = "200" ]; then
        local provider
        provider=$(get_header "X-DIN-Provider")
        print_test_result "Non-streaming - balanced tier" "PASS" "Provider: $provider"
    else
        print_test_result "Non-streaming - balanced tier" "FAIL" "Status: $LAST_STATUS_CODE, Body: $LAST_BODY"
    fi
}

# ============================================================
# Test: Streaming completions
# ============================================================
test_streaming() {
    echo -e "\n${CYAN}Testing Streaming Completions${NC}"

    local tmp_headers
    tmp_headers=$(mktemp)
    local tmp_body
    tmp_body=$(mktemp)

    local status
    status=$(curl -s -o "$tmp_body" -w "%{http_code}" -m "$TIMEOUT" \
        -D "$tmp_headers" \
        -H "Content-Type: application/json" \
        -H "X-DIN-Tier: fast" \
        -d '{"model":"ignored","messages":[{"role":"user","content":"Say exactly: hello"}],"stream":true,"max_tokens":10}' \
        "$BASE_URL/v1/chat/completions" 2>/dev/null)

    if [ "$status" = "200" ]; then
        local content_type
        content_type=$(grep -i "^content-type:" "$tmp_headers" | tr -d '\r')
        local provider
        provider=$(grep -i "^x-din-provider:" "$tmp_headers" | sed 's/^[^:]*: //' | tr -d '\r')
        local has_done
        has_done=$(grep -c "\[DONE\]" "$tmp_body" || true)

        if [ "$has_done" -ge 1 ]; then
            print_test_result "Streaming - fast tier" "PASS" "Provider: $provider, has [DONE] terminator"
        else
            print_test_result "Streaming - fast tier" "FAIL" "Missing [DONE] terminator in stream"
        fi
    else
        print_test_result "Streaming - fast tier" "FAIL" "Status: $status"
    fi

    rm -f "$tmp_headers" "$tmp_body"

    # Streaming balanced tier
    tmp_headers=$(mktemp)
    tmp_body=$(mktemp)

    status=$(curl -s -o "$tmp_body" -w "%{http_code}" -m "$TIMEOUT" \
        -D "$tmp_headers" \
        -H "Content-Type: application/json" \
        -H "X-DIN-Tier: balanced" \
        -d '{"model":"ignored","messages":[{"role":"user","content":"Say exactly: hello"}],"stream":true,"max_tokens":10}' \
        "$BASE_URL/v1/chat/completions" 2>/dev/null)

    if [ "$status" = "200" ]; then
        local provider
        provider=$(grep -i "^x-din-provider:" "$tmp_headers" | sed 's/^[^:]*: //' | tr -d '\r')
        print_test_result "Streaming - balanced tier" "PASS" "Provider: $provider"
    else
        print_test_result "Streaming - balanced tier" "FAIL" "Status: $status"
    fi

    rm -f "$tmp_headers" "$tmp_body"
}

# ============================================================
# Test: Response headers
# ============================================================
test_response_headers() {
    echo -e "\n${PURPLE}Testing Response Headers${NC}"

    make_chat_request "fast" "" "false"

    local provider tier model request_id
    provider=$(get_header "X-DIN-Provider")
    tier=$(get_header "X-DIN-Tier")
    model=$(get_header "X-DIN-Model")
    request_id=$(get_header "X-DIN-Request-Id")

    local all_present=true
    [ -z "$provider" ] && all_present=false
    [ -z "$tier" ] && all_present=false
    [ -z "$model" ] && all_present=false
    [ -z "$request_id" ] && all_present=false

    if [ "$all_present" = true ]; then
        print_test_result "Response headers present" "PASS" "Provider=$provider, Tier=$tier, Model=$model, RequestId=${request_id:0:8}..."
    else
        print_test_result "Response headers present" "FAIL" "Missing headers: Provider=$provider Tier=$tier Model=$model RequestId=$request_id"
    fi
}

# ============================================================
# Test: Session stickiness
# ============================================================
test_session_stickiness() {
    echo -e "\n${YELLOW}Testing Session Stickiness${NC}"

    local session_id="e2e-test-session-$(date +%s)"
    local providers=()

    for i in {1..3}; do
        make_chat_request "fast" "$session_id" "false"
        if [ "$LAST_STATUS_CODE" = "200" ]; then
            local provider
            provider=$(get_header "X-DIN-Provider")
            local pinned
            pinned=$(get_header "X-DIN-Session-Pinned")
            providers+=("$provider")

            if [ "$pinned" != "true" ]; then
                print_test_result "Session stickiness - pinned header" "FAIL" "X-DIN-Session-Pinned not set on request $i"
                return
            fi
        else
            print_test_result "Session stickiness" "FAIL" "Request $i returned $LAST_STATUS_CODE"
            return
        fi
    done

    # All 3 should hit the same provider
    if [ "${providers[0]}" = "${providers[1]}" ] && [ "${providers[1]}" = "${providers[2]}" ]; then
        print_test_result "Session stickiness" "PASS" "All 3 requests routed to ${providers[0]}"
    else
        print_test_result "Session stickiness" "FAIL" "Providers varied: ${providers[*]}"
    fi
}

# ============================================================
# Test: Tier selection
# ============================================================
test_tier_selection() {
    echo -e "\n${CYAN}Testing Tier Selection${NC}"

    for tier in fast balanced; do
        make_chat_request "$tier" "" "false"
        local resp_tier
        resp_tier=$(get_header "X-DIN-Tier")

        if [ "$LAST_STATUS_CODE" = "200" ] && [ "$resp_tier" = "$tier" ]; then
            print_test_result "Tier selection - $tier" "PASS" "Confirmed tier=$resp_tier"
        else
            print_test_result "Tier selection - $tier" "FAIL" "Status=$LAST_STATUS_CODE, Tier=$resp_tier (expected $tier)"
        fi
    done
}

# ============================================================
# Test: Error handling
# ============================================================
test_error_handling() {
    echo -e "\n${RED}Testing Error Handling${NC}"

    # Invalid content type
    local status
    status=$(curl -s -o /dev/null -w "%{http_code}" -m 10 \
        -H "Content-Type: text/plain" \
        -d "not json" \
        "$BASE_URL/v1/chat/completions" 2>/dev/null)

    if [ "$status" = "415" ]; then
        print_test_result "Invalid content type" "PASS" "Status: $status"
    else
        print_test_result "Invalid content type" "FAIL" "Expected 415, got $status"
    fi

    # Invalid JSON body
    status=$(curl -s -o /dev/null -w "%{http_code}" -m 10 \
        -H "Content-Type: application/json" \
        -d "{invalid json" \
        "$BASE_URL/v1/chat/completions" 2>/dev/null)

    if [ "$status" = "400" ]; then
        print_test_result "Invalid JSON body" "PASS" "Status: $status"
    else
        print_test_result "Invalid JSON body" "FAIL" "Expected 400, got $status"
    fi

    # Unknown tier
    local tmp_headers
    tmp_headers=$(mktemp)
    status=$(curl -s -o /dev/null -w "%{http_code}" -m 10 \
        -D "$tmp_headers" \
        -H "Content-Type: application/json" \
        -H "X-DIN-Tier: nonexistent" \
        -d '{"model":"x","messages":[{"role":"user","content":"hi"}]}' \
        "$BASE_URL/v1/chat/completions" 2>/dev/null)
    rm -f "$tmp_headers"

    if [ "$status" = "400" ]; then
        print_test_result "Unknown tier" "PASS" "Status: $status"
    else
        print_test_result "Unknown tier" "FAIL" "Expected 400, got $status"
    fi
}

# ============================================================
# Summary
# ============================================================
print_summary() {
    echo -e "\n${PURPLE}================================================${NC}"
    echo -e "${PURPLE}AI E2E Test Summary${NC}"
    echo -e "${PURPLE}================================================${NC}"

    echo -e "${BLUE}Total Tests: ${NC}$TOTAL_TESTS"
    echo -e "${GREEN}Passed: ${NC}$PASSED_TESTS"
    echo -e "${RED}Failed: ${NC}$FAILED_TESTS"

    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "\n${GREEN}All AI e2e tests passed.${NC}"
        exit 0
    else
        local success_rate=$((PASSED_TESTS * 100 / TOTAL_TESTS))
        echo -e "\n${RED}Some AI e2e tests failed.${NC}"
        echo -e "${YELLOW}Success Rate: ${NC}$success_rate%"
        exit 1
    fi
}

# ============================================================
# Main
# ============================================================
main() {
    echo -e "${CYAN}Starting AI Smart Routing E2E Tests${NC}"
    echo -e "${CYAN}Timestamp: $(date)${NC}"

    # Validate server is up
    echo -e "\n${CYAN}Validating test environment...${NC}"
    if ! curl -s -f "$BASE_URL/health" > /dev/null 2>&1; then
        echo -e "${RED}Cannot reach $BASE_URL/health${NC}"
        exit 1
    fi
    echo -e "${GREEN}Server is ready${NC}"

    test_health
    test_non_streaming
    test_streaming
    test_response_headers
    test_session_stickiness
    test_tier_selection
    test_error_handling

    print_summary
}

main "$@"
