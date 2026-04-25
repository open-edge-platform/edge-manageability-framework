---
name: cert-manager
description: Manage TLS certificates with cert-manager
---

You are managing certificates. Follow these steps:

1. **Certificate inventory**:
   - List all certificates: `kubectl get certificates -A`
   - Check certificate status and expiry
   - Review CertificateRequests: `kubectl get certificaterequests -A`
   - Identify failed or pending certificates

2. **Issuer validation**:
   - Check Issuer/ClusterIssuer status: `kubectl get issuers,clusterissuers -A`
   - Verify ACME/CA configuration
   - Test issuer connectivity
   - Review rate limits and quotas

3. **Certificate troubleshooting**:
   - Describe certificate for errors: `kubectl describe certificate <name>`
   - Check cert-manager pod logs
   - Verify DNS records for ACME challenges
   - Review secret creation and mounting

4. **Certificate operations**:
   - Create Certificate resources with proper annotations
   - Configure renewal settings (renewBefore)
   - Set up Certificate issuance for Ingress
   - Test certificate renewal process

5. **Security best practices**:
   - Use appropriate key sizes
   - Configure proper SANs (Subject Alternative Names)
   - Implement certificate rotation
   - Monitor expiry dates
   - Document certificate dependencies
