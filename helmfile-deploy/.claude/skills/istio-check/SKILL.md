---
name: istio-check
description: Validate Istio service mesh configuration and health
---

You are checking Istio configuration. Follow these steps:

1. **Control plane health**:
   - Check Istiod status: `kubectl get pods -n istio-system`
   - Review Istiod logs for errors
   - Verify pilot is syncing: `istioctl proxy-status`

2. **Sidecar injection**:
   - Check namespace labels: `kubectl get ns --show-labels`
   - Verify pods have envoy sidecars: `kubectl get pods -o jsonpath='{.items[*].spec.containers[*].name}'`
   - Review injection webhook configuration

3. **Traffic management**:
   - Validate VirtualServices: `kubectl get virtualservices -A`
   - Check DestinationRules: `kubectl get destinationrules -A`
   - Verify Gateways: `kubectl get gateways -A`
   - Test routing rules with `istioctl analyze`

4. **Security policies**:
   - Review PeerAuthentication policies
   - Check AuthorizationPolicies
   - Verify mTLS status: `istioctl x describe pod <pod>`

5. **Troubleshooting**:
   - Check proxy logs: `kubectl logs <pod> -c istio-proxy`
   - Review config: `istioctl proxy-config routes <pod>`
   - Test connectivity between services
   - Report issues with suggested fixes
