#!/bin/bash
# =============================================================================
# Container Security Testing Script
# =============================================================================
# Tests container escape techniques and verifies security controls
# Reference: https://medium.com article on Docker privilege escalation
#
# Tools tested:
# - deepce (Docker Enumeration, Escalation of Privileges and Container Escapes)
# - amicontained (Container introspection)
# - Manual escape technique checks
#
# Usage: kubectl exec -it <pod> -- /bin/sh -c "$(cat security-test.sh)"
# Or:    ./security-test.sh (run inside container)
# =============================================================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_header() {
    echo ""
    echo -e "${BLUE}======================================================================${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}======================================================================${NC}"
}

print_test() {
    echo -e "\n${YELLOW}[TEST]${NC} $1"
}

print_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

print_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
}

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

# =============================================================================
print_header "CONTAINER ENVIRONMENT DETECTION"
# =============================================================================

print_test "Detecting container runtime..."

# Check if we're in a container
if [ -f /.dockerenv ]; then
    print_info "Running inside Docker container (/.dockerenv exists)"
elif grep -q docker /proc/1/cgroup 2>/dev/null; then
    print_info "Running inside Docker container (cgroup detection)"
elif grep -q kubepods /proc/1/cgroup 2>/dev/null; then
    print_info "Running inside Kubernetes pod (kubepods in cgroup)"
else
    print_info "May not be running in a container"
fi

# =============================================================================
print_header "SECURITY CONTROL VERIFICATION"
# =============================================================================

ESCAPE_POSSIBLE=0

# -----------------------------------------------------------------------------
print_test "1. Checking if running as root..."
# -----------------------------------------------------------------------------
if [ "$(id -u)" -eq 0 ]; then
    print_fail "Running as root (UID 0) - DANGEROUS"
    ESCAPE_POSSIBLE=1
else
    print_pass "Running as non-root user (UID: $(id -u))"
fi

# -----------------------------------------------------------------------------
print_test "2. Checking filesystem writability..."
# -----------------------------------------------------------------------------
if touch /test_write 2>/dev/null; then
    rm /test_write
    print_fail "Root filesystem is writable - Can drop malicious files"
    ESCAPE_POSSIBLE=1
else
    print_pass "Root filesystem is read-only"
fi

# Check tmp directories
for dir in /tmp /app/tmp; do
    if [ -d "$dir" ] && touch "$dir/test_write" 2>/dev/null; then
        rm "$dir/test_write"
        print_info "$dir is writable (expected for temp files)"
    fi
done

# -----------------------------------------------------------------------------
print_test "3. Checking for dangerous capabilities..."
# -----------------------------------------------------------------------------
# CAP_SYS_ADMIN - Most dangerous, allows mount, namespace manipulation
# CAP_SYS_PTRACE - Process tracing, can inject code
# CAP_SYS_MODULE - Kernel module loading
# CAP_NET_ADMIN - Network manipulation
# CAP_DAC_OVERRIDE - Bypass file permission checks

if command -v capsh &> /dev/null; then
    CAPS=$(capsh --print 2>/dev/null | grep "Current:" || echo "")
    if echo "$CAPS" | grep -q "cap_sys_admin"; then
        print_fail "CAP_SYS_ADMIN present - Container escape via mount possible"
        ESCAPE_POSSIBLE=1
    else
        print_pass "CAP_SYS_ADMIN not present"
    fi

    if echo "$CAPS" | grep -q "cap_sys_ptrace"; then
        print_fail "CAP_SYS_PTRACE present - Process injection possible"
        ESCAPE_POSSIBLE=1
    else
        print_pass "CAP_SYS_PTRACE not present"
    fi

    if echo "$CAPS" | grep -q "cap_sys_module"; then
        print_fail "CAP_SYS_MODULE present - Kernel module loading possible"
        ESCAPE_POSSIBLE=1
    else
        print_pass "CAP_SYS_MODULE not present"
    fi
elif [ -f /proc/self/status ]; then
    CAP_EFF=$(grep CapEff /proc/self/status | awk '{print $2}')
    print_info "Effective capabilities (hex): $CAP_EFF"
    if [ "$CAP_EFF" = "0000000000000000" ]; then
        print_pass "No effective capabilities"
    else
        print_info "Some capabilities present - review manually"
    fi
else
    print_info "Cannot determine capabilities (capsh not available)"
fi

# -----------------------------------------------------------------------------
print_test "4. Checking for Docker socket access..."
# -----------------------------------------------------------------------------
# Docker socket = full host compromise
DOCKER_SOCK="/var/run/docker.sock"
if [ -S "$DOCKER_SOCK" ]; then
    print_fail "Docker socket mounted at $DOCKER_SOCK - CRITICAL: Full host access possible"
    ESCAPE_POSSIBLE=1
else
    print_pass "Docker socket not mounted"
fi

# Check for TCP docker
for port in 2375 2376; do
    if command -v nc &> /dev/null && nc -z localhost $port 2>/dev/null; then
        print_fail "Docker API accessible on port $port - CRITICAL"
        ESCAPE_POSSIBLE=1
    fi
done

# -----------------------------------------------------------------------------
print_test "5. Checking for privileged mode..."
# -----------------------------------------------------------------------------
if [ -w /sys/kernel ]; then
    print_fail "Container appears to be running in privileged mode"
    ESCAPE_POSSIBLE=1
else
    print_pass "Container is not privileged"
fi

# -----------------------------------------------------------------------------
print_test "6. Checking for host namespace sharing..."
# -----------------------------------------------------------------------------
# PID namespace
if [ "$(ls /proc | grep -E '^[0-9]+$' | wc -l)" -gt 50 ]; then
    print_fail "Many processes visible - may be sharing host PID namespace"
    ESCAPE_POSSIBLE=1
else
    print_pass "PID namespace appears isolated"
fi

# Network namespace check
if [ -f /proc/net/route ]; then
    ROUTES=$(cat /proc/net/route | wc -l)
    if [ "$ROUTES" -gt 10 ]; then
        print_info "Multiple network routes - check if host network is shared"
    fi
fi

# -----------------------------------------------------------------------------
print_test "7. Checking for sensitive mounts..."
# -----------------------------------------------------------------------------
SENSITIVE_PATHS=(
    "/etc/shadow"
    "/etc/passwd"
    "/root"
    "/home"
    "/etc/kubernetes"
    "/var/lib/kubelet"
    "/etc/cni"
)

for path in "${SENSITIVE_PATHS[@]}"; do
    if [ -r "$path" ] && [ -f "$path" ] 2>/dev/null; then
        print_fail "Sensitive path readable: $path"
    fi
done

# Check for host filesystem mounts
if mount | grep -q "type ext4\|type xfs" 2>/dev/null; then
    print_info "Host filesystem mounts detected - review mount list"
fi

# -----------------------------------------------------------------------------
print_test "8. Checking seccomp status..."
# -----------------------------------------------------------------------------
if [ -f /proc/self/status ]; then
    SECCOMP=$(grep Seccomp /proc/self/status | awk '{print $2}')
    case $SECCOMP in
        0) print_fail "Seccomp disabled - All syscalls allowed"
           ESCAPE_POSSIBLE=1 ;;
        1) print_info "Seccomp in strict mode" ;;
        2) print_pass "Seccomp filter active" ;;
        *) print_info "Seccomp status unknown: $SECCOMP" ;;
    esac
fi

# -----------------------------------------------------------------------------
print_test "9. Testing dangerous syscalls..."
# -----------------------------------------------------------------------------

# Test mount syscall
print_info "Attempting mount syscall..."
if mount -t tmpfs none /tmp/test_mount 2>/dev/null; then
    umount /tmp/test_mount 2>/dev/null
    print_fail "mount syscall allowed - Container escape possible"
    ESCAPE_POSSIBLE=1
else
    print_pass "mount syscall blocked"
fi

# Test unshare syscall (namespace creation)
print_info "Attempting unshare syscall..."
if unshare --user true 2>/dev/null; then
    print_fail "unshare syscall allowed - Namespace escape possible"
    ESCAPE_POSSIBLE=1
else
    print_pass "unshare syscall blocked"
fi

# -----------------------------------------------------------------------------
print_test "10. Checking for Kubernetes service account token..."
# -----------------------------------------------------------------------------
SA_TOKEN="/var/run/secrets/kubernetes.io/serviceaccount/token"
if [ -f "$SA_TOKEN" ]; then
    print_fail "Service account token mounted - Can interact with K8s API"
    print_info "Token path: $SA_TOKEN"
else
    print_pass "Service account token not mounted"
fi

# =============================================================================
print_header "AUTOMATED TOOL SIMULATION"
# =============================================================================

print_info "The following tools would be used by attackers:"
print_info ""
print_info "1. deepce - Docker enumeration and escape"
print_info "   curl -sL https://github.com/stealthcopter/deepce/raw/main/deepce.sh | sh"
print_info ""
print_info "2. amicontained - Container introspection"
print_info "   curl -sL https://github.com/genuinetools/amicontained/releases/download/v0.4.9/amicontained-linux-amd64 -o amicontained"
print_info ""
print_info "3. CDK - Container penetration toolkit"
print_info "   curl -sL https://github.com/cdk-team/CDK/releases/latest/download/cdk_linux_amd64 -o cdk"
print_info ""
print_info "4. linpeas - Linux privilege escalation"
print_info "   curl -sL https://github.com/peass-ng/PEASS-ng/releases/latest/download/linpeas.sh | sh"

# =============================================================================
print_header "ESCAPE TECHNIQUE TESTS"
# =============================================================================

print_test "Testing CVE-2019-5736 prerequisites (runc escape)..."
if [ -x /proc/self/exe ] && [ -w /proc/self/exe ] 2>/dev/null; then
    print_fail "CVE-2019-5736 may be exploitable"
    ESCAPE_POSSIBLE=1
else
    print_pass "CVE-2019-5736 prerequisites not met"
fi

print_test "Testing release_agent escape (cgroups)..."
CGROUP_RELEASE="/sys/fs/cgroup/*/release_agent"
if ls $CGROUP_RELEASE 2>/dev/null | head -1 | xargs -I {} test -w {}; then
    print_fail "Cgroup release_agent writable - Escape possible"
    ESCAPE_POSSIBLE=1
else
    print_pass "Cgroup release_agent not writable"
fi

print_test "Testing core_pattern escape..."
if [ -w /proc/sys/kernel/core_pattern ]; then
    print_fail "core_pattern writable - Escape possible"
    ESCAPE_POSSIBLE=1
else
    print_pass "core_pattern not writable"
fi

# =============================================================================
print_header "SUMMARY"
# =============================================================================

echo ""
if [ $ESCAPE_POSSIBLE -eq 0 ]; then
    print_pass "Container appears to be properly hardened!"
    echo ""
    echo "Security controls verified:"
    echo "  - Non-root user"
    echo "  - Read-only filesystem"
    echo "  - Capabilities dropped"
    echo "  - No Docker socket access"
    echo "  - Not privileged"
    echo "  - Namespaces isolated"
    echo "  - Seccomp enabled"
    echo "  - Service account token not mounted"
    echo ""
    echo "This container follows the security best practices from:"
    echo "  - https://www.alexpruteanu.cloud/blog/your-app-got-hacked-now-what"
    echo "  - https://www.alexpruteanu.cloud/blog/kubernetes-best-practices"
else
    print_fail "Container has security weaknesses! Review the FAIL items above."
    echo ""
    echo "Recommended actions:"
    echo "  1. Run container as non-root user"
    echo "  2. Use read-only root filesystem"
    echo "  3. Drop all capabilities"
    echo "  4. Never mount Docker socket"
    echo "  5. Don't use privileged mode"
    echo "  6. Enable seccomp profile"
    echo "  7. Use distroless images"
fi

echo ""
echo "Test completed at: $(date)"
