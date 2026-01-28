#!/bin/bash
# =============================================================================
# Helm Chart Test Runner
# =============================================================================
# Runs all tests for the goapp-stack Helm chart
#
# Tests include:
# 1. helm lint - Basic chart linting
# 2. helm template - Template rendering validation
# 3. helm unittest - Unit tests for templates
# 4. kubeconform - Kubernetes schema validation
# 5. Security policy checks
#
# Prerequisites:
# - helm (v3+)
# - helm-unittest plugin: helm plugin install https://github.com/helm-unittest/helm-unittest
# - kubeconform (optional): https://github.com/yannh/kubeconform
#
# Usage: ./run-tests.sh [--verbose] [--skip-kubeconform]
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHART_DIR="$(dirname "$SCRIPT_DIR")"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

VERBOSE=false
SKIP_KUBECONFORM=false
TESTS_PASSED=0
TESTS_FAILED=0

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --verbose|-v) VERBOSE=true; shift ;;
        --skip-kubeconform) SKIP_KUBECONFORM=true; shift ;;
        *) shift ;;
    esac
done

print_header() {
    echo ""
    echo -e "${BLUE}======================================================================${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}======================================================================${NC}"
}

print_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    TESTS_PASSED=$((TESTS_PASSED + 1))
}

print_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    TESTS_FAILED=$((TESTS_FAILED + 1))
}

print_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

# =============================================================================
print_header "1. HELM LINT"
# =============================================================================

if helm lint "$CHART_DIR" 2>&1; then
    print_pass "Helm lint passed"
else
    print_fail "Helm lint failed"
fi

# =============================================================================
print_header "2. TEMPLATE RENDERING"
# =============================================================================

# Test default values
if helm template test-release "$CHART_DIR" > /dev/null 2>&1; then
    print_pass "Template renders with default values"
else
    print_fail "Template rendering failed with default values"
fi

# Test with dev values
if [ -f "$CHART_DIR/values-dev.yaml" ]; then
    if helm template test-release "$CHART_DIR" -f "$CHART_DIR/values-dev.yaml" > /dev/null 2>&1; then
        print_pass "Template renders with dev values"
    else
        print_fail "Template rendering failed with dev values"
    fi
fi

# Test with prod values
if [ -f "$CHART_DIR/values-prod.yaml" ]; then
    if helm template test-release "$CHART_DIR" -f "$CHART_DIR/values-prod.yaml" > /dev/null 2>&1; then
        print_pass "Template renders with prod values"
    else
        print_fail "Template rendering failed with prod values"
    fi
fi

# Test with components disabled
if helm template test-release "$CHART_DIR" \
    --set postgresql.enabled=false \
    --set redis.enabled=false \
    --set elasticsearch.enabled=false \
    --set rabbitmq.enabled=false \
    --set gateway.enabled=false > /dev/null 2>&1; then
    print_pass "Template renders with all components disabled"
else
    print_fail "Template rendering failed with components disabled"
fi

# =============================================================================
print_header "3. HELM UNIT TESTS"
# =============================================================================

if helm plugin list | grep -q unittest; then
    if helm unittest "$CHART_DIR" 2>&1; then
        print_pass "Helm unit tests passed"
    else
        print_fail "Helm unit tests failed"
    fi
else
    print_info "helm-unittest plugin not installed, skipping..."
    print_info "Install with: helm plugin install https://github.com/helm-unittest/helm-unittest"
fi

# =============================================================================
print_header "4. KUBERNETES SCHEMA VALIDATION"
# =============================================================================

if [ "$SKIP_KUBECONFORM" = true ]; then
    print_info "Skipping kubeconform (--skip-kubeconform flag)"
elif command -v kubeconform &> /dev/null; then
    TEMP_DIR=$(mktemp -d)
    helm template test-release "$CHART_DIR" > "$TEMP_DIR/manifests.yaml" 2>/dev/null

    # Skip CRDs that kubeconform doesn't know about
    if kubeconform -summary -skip "Cluster,RedisFailover,Elasticsearch,RabbitmqCluster,Gateway,HTTPRoute" \
        "$TEMP_DIR/manifests.yaml" 2>&1; then
        print_pass "Kubernetes schema validation passed"
    else
        print_fail "Kubernetes schema validation failed"
    fi
    rm -rf "$TEMP_DIR"
else
    print_info "kubeconform not installed, skipping..."
    print_info "Install from: https://github.com/yannh/kubeconform"
fi

# =============================================================================
print_header "5. SECURITY POLICY CHECKS"
# =============================================================================

MANIFESTS=$(helm template test-release "$CHART_DIR" 2>/dev/null)

# Check for runAsNonRoot
if echo "$MANIFESTS" | grep -q "runAsNonRoot: true"; then
    print_pass "runAsNonRoot is enabled"
else
    print_fail "runAsNonRoot not found"
fi

# Check for readOnlyRootFilesystem
if echo "$MANIFESTS" | grep -q "readOnlyRootFilesystem: true"; then
    print_pass "readOnlyRootFilesystem is enabled"
else
    print_fail "readOnlyRootFilesystem not found"
fi

# Check for allowPrivilegeEscalation
if echo "$MANIFESTS" | grep -q "allowPrivilegeEscalation: false"; then
    print_pass "allowPrivilegeEscalation is disabled"
else
    print_fail "allowPrivilegeEscalation: false not found"
fi

# Check for capabilities drop ALL
if echo "$MANIFESTS" | grep -A1 "drop:" | grep -q "ALL"; then
    print_pass "All capabilities are dropped"
else
    print_fail "capabilities drop ALL not found"
fi

# Check for seccomp profile
if echo "$MANIFESTS" | grep -q "seccompProfile"; then
    print_pass "Seccomp profile is configured"
else
    print_fail "Seccomp profile not found"
fi

# Check for automountServiceAccountToken
if echo "$MANIFESTS" | grep -q "automountServiceAccountToken: false"; then
    print_pass "Service account token auto-mount is disabled"
else
    print_fail "automountServiceAccountToken: false not found"
fi

# Check for Pod Security Standards
if echo "$MANIFESTS" | grep -q "pod-security.kubernetes.io/enforce"; then
    print_pass "Pod Security Standards labels present"
else
    print_fail "Pod Security Standards labels not found"
fi

# Check for NetworkPolicy
if echo "$MANIFESTS" | grep -q "kind: NetworkPolicy"; then
    print_pass "NetworkPolicy is configured"
else
    print_fail "NetworkPolicy not found"
fi

# Check for PodDisruptionBudget
if echo "$MANIFESTS" | grep -q "kind: PodDisruptionBudget"; then
    print_pass "PodDisruptionBudget is configured"
else
    print_fail "PodDisruptionBudget not found"
fi

# Check for resource limits
if echo "$MANIFESTS" | grep -q "limits:"; then
    print_pass "Resource limits are configured"
else
    print_fail "Resource limits not found"
fi

# =============================================================================
print_header "6. VALUES VALIDATION"
# =============================================================================

# Check values.yaml exists and is valid YAML
if [ -f "$CHART_DIR/values.yaml" ]; then
    if helm template test-release "$CHART_DIR" --debug 2>&1 | grep -q "Error"; then
        print_fail "values.yaml has errors"
    else
        print_pass "values.yaml is valid"
    fi
fi

# Check for default secret that should be changed
if grep -q "changeme" "$CHART_DIR/values.yaml" 2>/dev/null; then
    print_info "Default secrets found in values.yaml (expected - should be overridden in production)"
fi

# =============================================================================
print_header "TEST SUMMARY"
# =============================================================================

echo ""
echo "========================================================================"
echo -e "Tests Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Tests Failed: ${RED}$TESTS_FAILED${NC}"
echo "========================================================================"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed. Please review the output above.${NC}"
    exit 1
fi
