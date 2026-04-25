---
name: helm-deploy
description: Deploy Helm charts with validation and health checks
---

You are deploying a Helm chart. Follow these steps:

1. **Validate the chart first**:
   - Run `helm lint` on the chart directory
   - Check for required values in values.yaml
   - Validate YAML syntax

2. **Dry-run before deployment**:
   - Execute `helm install --dry-run --debug` to preview
   - Review the rendered manifests
   - Check for any template errors

3. **Deploy with monitoring**:
   - Use `helm install` or `helm upgrade --install`
   - Add `--wait` and `--timeout 5m` flags
   - Monitor pod status with `kubectl get pods -w`

4. **Post-deployment validation**:
   - Verify all pods are running
   - Check service endpoints
   - Test basic connectivity
   - Run `helm status` to confirm

5. **Report results** including:
   - Deployment status
   - Resource summary
   - Any warnings or errors
   - Next steps or rollback commands if needed
