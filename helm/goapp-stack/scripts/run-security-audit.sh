#!/bin/bash
# =============================================================================
# Security Audit Runner
# =============================================================================
# Runs security tests against deployed containers using automated tools
#
# Prerequisites:
# - kubectl configured with cluster access
# - Helm release deployed
#
# Usage: ./run-security-audit.sh [namespace] [release-name]
# =============================================================================

set -e

NAMESPACE=${1:-goapp}
RELEASE=${2:-goapp}

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}======================================================================${NC}"
echo -e "${BLUE}  Container Security Audit - $RELEASE in $NAMESPACE${NC}"
echo -e "${BLUE}======================================================================${NC}"

# Check if namespace exists
if ! kubectl get namespace "$NAMESPACE" &>/dev/null; then
    echo -e "${RED}[ERROR]${NC} Namespace $NAMESPACE does not exist"
    exit 1
fi

# Get a running pod
POD=$(kubectl get pods -n "$NAMESPACE" -l "app=goapp" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -z "$POD" ]; then
    echo -e "${RED}[ERROR]${NC} No running pods found with label app=goapp"
    exit 1
fi

echo -e "${GREEN}[INFO]${NC} Found pod: $POD"
echo ""

# =============================================================================
echo -e "${YELLOW}[1/6]${NC} Running basic security checks..."
# =============================================================================

kubectl exec -n "$NAMESPACE" "$POD" -- sh -c '
echo "=== User Context ==="
id
echo ""
echo "=== Filesystem Check ==="
touch /test_write 2>&1 || echo "Root filesystem is read-only (GOOD)"
rm /test_write 2>/dev/null
echo ""
echo "=== Capabilities ==="
cat /proc/self/status | grep -E "Cap(Inh|Prm|Eff|Bnd|Amb)"
echo ""
echo "=== Seccomp Status ==="
cat /proc/self/status | grep Seccomp
' 2>/dev/null || echo "Basic checks completed (some commands may not be available)"

# =============================================================================
echo ""
echo -e "${YELLOW}[2/6]${NC} Checking for dangerous mounts..."
# =============================================================================

kubectl exec -n "$NAMESPACE" "$POD" -- sh -c '
echo "=== Mount Points ==="
mount 2>/dev/null | head -20 || cat /proc/mounts | head -20
echo ""
echo "=== Docker Socket Check ==="
ls -la /var/run/docker.sock 2>&1 || echo "Docker socket not mounted (GOOD)"
echo ""
echo "=== Service Account Token ==="
ls -la /var/run/secrets/kubernetes.io/serviceaccount/ 2>&1 || echo "SA token not mounted (GOOD)"
' 2>/dev/null || echo "Mount checks completed"

# =============================================================================
echo ""
echo -e "${YELLOW}[3/6]${NC} Testing syscall restrictions..."
# =============================================================================

kubectl exec -n "$NAMESPACE" "$POD" -- sh -c '
echo "=== Testing mount syscall ==="
mount -t tmpfs none /tmp/test 2>&1 || echo "mount blocked (GOOD)"
umount /tmp/test 2>/dev/null

echo ""
echo "=== Testing unshare syscall ==="
unshare --user true 2>&1 || echo "unshare blocked (GOOD)"
' 2>/dev/null || echo "Syscall tests completed"

# =============================================================================
echo ""
echo -e "${YELLOW}[4/6]${NC} Checking Pod Security Context..."
# =============================================================================

echo "Pod spec security settings:"
kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='
Security Context:
  runAsNonRoot: {.spec.securityContext.runAsNonRoot}
  runAsUser: {.spec.securityContext.runAsUser}
  fsGroup: {.spec.securityContext.fsGroup}
  seccompProfile: {.spec.securityContext.seccompProfile.type}

Container Security Context:
  allowPrivilegeEscalation: {.spec.containers[0].securityContext.allowPrivilegeEscalation}
  privileged: {.spec.containers[0].securityContext.privileged}
  readOnlyRootFilesystem: {.spec.containers[0].securityContext.readOnlyRootFilesystem}
  capabilities.drop: {.spec.containers[0].securityContext.capabilities.drop}
'
echo ""

# =============================================================================
echo ""
echo -e "${YELLOW}[5/6]${NC} Checking Namespace Pod Security Standards..."
# =============================================================================

echo "Namespace labels:"
kubectl get namespace "$NAMESPACE" -o jsonpath='{.metadata.labels}' | tr ',' '\n' | grep -E "pod-security|enforce|warn|audit" || echo "No PSS labels found"
echo ""

# =============================================================================
echo ""
echo -e "${YELLOW}[6/6]${NC} Checking NetworkPolicy..."
# =============================================================================

echo "NetworkPolicies in namespace:"
kubectl get networkpolicies -n "$NAMESPACE" -o wide 2>/dev/null || echo "No NetworkPolicies found"
echo ""

# =============================================================================
echo ""
echo -e "${BLUE}======================================================================${NC}"
echo -e "${BLUE}  AUDIT SUMMARY${NC}"
echo -e "${BLUE}======================================================================${NC}"
# =============================================================================

# Collect results
ISSUES=0

# Check if running as root
ROOT_CHECK=$(kubectl exec -n "$NAMESPACE" "$POD" -- id -u 2>/dev/null)
if [ "$ROOT_CHECK" = "0" ]; then
    echo -e "${RED}[FAIL]${NC} Container running as root"
    ISSUES=$((ISSUES + 1))
else
    echo -e "${GREEN}[PASS]${NC} Container running as non-root (UID: $ROOT_CHECK)"
fi

# Check readOnlyRootFilesystem
RO_FS=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.spec.containers[0].securityContext.readOnlyRootFilesystem}')
if [ "$RO_FS" = "true" ]; then
    echo -e "${GREEN}[PASS]${NC} Read-only root filesystem enabled"
else
    echo -e "${RED}[FAIL]${NC} Root filesystem is writable"
    ISSUES=$((ISSUES + 1))
fi

# Check allowPrivilegeEscalation
PRIV_ESC=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.spec.containers[0].securityContext.allowPrivilegeEscalation}')
if [ "$PRIV_ESC" = "false" ]; then
    echo -e "${GREEN}[PASS]${NC} Privilege escalation disabled"
else
    echo -e "${RED}[FAIL]${NC} Privilege escalation allowed"
    ISSUES=$((ISSUES + 1))
fi

# Check capabilities
CAPS_DROP=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.spec.containers[0].securityContext.capabilities.drop}')
if echo "$CAPS_DROP" | grep -q "ALL"; then
    echo -e "${GREEN}[PASS]${NC} All capabilities dropped"
else
    echo -e "${RED}[FAIL]${NC} Not all capabilities dropped"
    ISSUES=$((ISSUES + 1))
fi

# Check seccomp
SECCOMP=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.spec.securityContext.seccompProfile.type}')
if [ -n "$SECCOMP" ]; then
    echo -e "${GREEN}[PASS]${NC} Seccomp profile: $SECCOMP"
else
    echo -e "${YELLOW}[WARN]${NC} No seccomp profile specified"
fi

# Check automountServiceAccountToken
SA_MOUNT=$(kubectl get pod -n "$NAMESPACE" "$POD" -o jsonpath='{.spec.automountServiceAccountToken}')
if [ "$SA_MOUNT" = "false" ]; then
    echo -e "${GREEN}[PASS]${NC} Service account token not auto-mounted"
else
    echo -e "${YELLOW}[WARN]${NC} Service account token may be mounted"
fi

# Check PSS
PSS_ENFORCE=$(kubectl get namespace "$NAMESPACE" -o jsonpath='{.metadata.labels.pod-security\.kubernetes\.io/enforce}' 2>/dev/null)
if [ "$PSS_ENFORCE" = "restricted" ]; then
    echo -e "${GREEN}[PASS]${NC} Pod Security Standards: restricted"
elif [ -n "$PSS_ENFORCE" ]; then
    echo -e "${YELLOW}[WARN]${NC} Pod Security Standards: $PSS_ENFORCE"
else
    echo -e "${RED}[FAIL]${NC} No Pod Security Standards enforced"
    ISSUES=$((ISSUES + 1))
fi

# Check NetworkPolicy
NP_COUNT=$(kubectl get networkpolicies -n "$NAMESPACE" -o json 2>/dev/null | grep -c '"name"' || echo "0")
if [ "$NP_COUNT" -gt 0 ]; then
    echo -e "${GREEN}[PASS]${NC} NetworkPolicy configured ($NP_COUNT policies)"
else
    echo -e "${RED}[FAIL]${NC} No NetworkPolicy configured"
    ISSUES=$((ISSUES + 1))
fi

echo ""
echo "========================================================================"
if [ $ISSUES -eq 0 ]; then
    echo -e "${GREEN}All security checks passed!${NC}"
    echo "Container is well-hardened against escape techniques."
else
    echo -e "${RED}Found $ISSUES security issue(s)${NC}"
    echo "Review the FAIL items above and update your deployment."
fi
echo "========================================================================"
echo ""
echo "For deeper testing, run inside the container:"
echo "  kubectl exec -it -n $NAMESPACE $POD -- sh"
echo "  # Then run manual escape attempts"
echo ""
echo "Audit completed at: $(date)"
