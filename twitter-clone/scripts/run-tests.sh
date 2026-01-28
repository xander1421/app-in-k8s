#!/bin/bash

# Test runner script for Twitter Clone
# Usage: ./run-tests.sh [unit|integration|e2e|all]

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
TEST_TYPE=${1:-all}
COVERAGE_DIR="./coverage"
COVERAGE_THRESHOLD=70

echo -e "${YELLOW}Running Twitter Clone Tests${NC}"
echo "Test Type: $TEST_TYPE"
echo ""

# Create coverage directory
mkdir -p $COVERAGE_DIR

# Function to run unit tests
run_unit_tests() {
    echo -e "${GREEN}Running Unit Tests...${NC}"
    
    # Test each service
    services=(
        "services/user-service"
        "services/tweet-service"
        "services/timeline-service"
        "services/media-service"
        "services/notification-service"
        "services/search-service"
        "services/fanout-service"
    )
    
    for service in "${services[@]}"; do
        echo "Testing $service..."
        cd $service
        go test -v -race -coverprofile=$COVERAGE_DIR/$(basename $service).coverage ./...
        cd ../..
    done
    
    # Test packages
    echo "Testing pkg..."
    cd pkg
    go test -v -race -coverprofile=$COVERAGE_DIR/pkg.coverage ./...
    cd ..
}

# Function to run integration tests
run_integration_tests() {
    echo -e "${GREEN}Running Integration Tests...${NC}"
    
    # Start test dependencies
    echo "Starting test dependencies..."
    docker-compose -f tests/docker-compose.test.yml up -d
    
    # Wait for services
    sleep 10
    
    # Run integration tests
    go test -v -tags=integration ./tests/integration/... -coverprofile=$COVERAGE_DIR/integration.coverage
    
    # Stop test dependencies
    docker-compose -f tests/docker-compose.test.yml down
}

# Function to run E2E tests
run_e2e_tests() {
    echo -e "${GREEN}Running End-to-End Tests...${NC}"
    
    # Start all services
    echo "Starting services..."
    ./scripts/deploy.sh test
    
    # Wait for services to be ready
    echo "Waiting for services..."
    sleep 30
    
    # Run E2E tests
    go test -v -tags=e2e ./tests/e2e/... -timeout 30m
    
    # Stop services
    kubectl delete namespace twitter-test || true
}

# Function to generate coverage report
generate_coverage_report() {
    echo -e "${GREEN}Generating Coverage Report...${NC}"
    
    # Merge coverage files
    echo "mode: set" > $COVERAGE_DIR/combined.coverage
    tail -q -n +2 $COVERAGE_DIR/*.coverage >> $COVERAGE_DIR/combined.coverage
    
    # Generate HTML report
    go tool cover -html=$COVERAGE_DIR/combined.coverage -o $COVERAGE_DIR/coverage.html
    
    # Calculate total coverage
    TOTAL_COVERAGE=$(go tool cover -func=$COVERAGE_DIR/combined.coverage | grep total | awk '{print $3}' | sed 's/%//')
    
    echo "Total Coverage: ${TOTAL_COVERAGE}%"
    
    # Check threshold
    if (( $(echo "$TOTAL_COVERAGE < $COVERAGE_THRESHOLD" | bc -l) )); then
        echo -e "${RED}Coverage ${TOTAL_COVERAGE}% is below threshold ${COVERAGE_THRESHOLD}%${NC}"
        exit 1
    else
        echo -e "${GREEN}Coverage ${TOTAL_COVERAGE}% meets threshold ${COVERAGE_THRESHOLD}%${NC}"
    fi
}

# Function to run load tests
run_load_tests() {
    echo -e "${GREEN}Running Load Tests...${NC}"
    
    # Check if k6 is installed
    if ! command -v k6 &> /dev/null; then
        echo "k6 not found. Installing..."
        brew install k6 || apt-get install k6 || yum install k6
    fi
    
    # Run load tests
    k6 run tests/load/timeline_load_test.js
    k6 run tests/load/tweet_load_test.js
    k6 run tests/load/search_load_test.js
}

# Function to run security tests
run_security_tests() {
    echo -e "${GREEN}Running Security Tests...${NC}"
    
    # Check for common vulnerabilities
    echo "Checking for vulnerabilities..."
    go list -json -m all | nancy sleuth
    
    # Run gosec for security analysis
    if command -v gosec &> /dev/null; then
        gosec ./...
    else
        echo "gosec not found, skipping security scan"
    fi
    
    # Check for secrets
    if command -v gitleaks &> /dev/null; then
        gitleaks detect --source . --verbose
    else
        echo "gitleaks not found, skipping secret scan"
    fi
}

# Function to run linters
run_linters() {
    echo -e "${GREEN}Running Linters...${NC}"
    
    # Run golangci-lint
    if command -v golangci-lint &> /dev/null; then
        golangci-lint run ./...
    else
        echo "Installing golangci-lint..."
        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
        golangci-lint run ./...
    fi
    
    # Run staticcheck
    if command -v staticcheck &> /dev/null; then
        staticcheck ./...
    else
        go install honnef.co/go/tools/cmd/staticcheck@latest
        staticcheck ./...
    fi
}

# Main execution
case $TEST_TYPE in
    unit)
        run_unit_tests
        generate_coverage_report
        ;;
    integration)
        run_integration_tests
        generate_coverage_report
        ;;
    e2e)
        run_e2e_tests
        ;;
    load)
        run_load_tests
        ;;
    security)
        run_security_tests
        ;;
    lint)
        run_linters
        ;;
    all)
        run_linters
        run_unit_tests
        run_integration_tests
        run_security_tests
        generate_coverage_report
        ;;
    *)
        echo -e "${RED}Unknown test type: $TEST_TYPE${NC}"
        echo "Usage: $0 [unit|integration|e2e|load|security|lint|all]"
        exit 1
        ;;
esac

echo ""
echo -e "${GREEN}Tests completed successfully!${NC}"

# Open coverage report if on macOS
if [[ "$OSTYPE" == "darwin"* ]] && [[ -f "$COVERAGE_DIR/coverage.html" ]]; then
    echo "Opening coverage report..."
    open $COVERAGE_DIR/coverage.html
fi