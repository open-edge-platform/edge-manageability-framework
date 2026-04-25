---
name: ci-cd-pipeline
description: Build and optimize CI/CD pipelines
---

You are working on CI/CD pipelines. Follow these steps:

1. **Pipeline design**:
   - Define stages: build, test, security scan, deploy
   - Identify parallel vs sequential jobs
   - Set appropriate timeouts
   - Plan for rollback scenarios

2. **Build optimization**:
   - Use layer caching for Docker builds
   - Implement build artifact caching
   - Parallelize independent tasks
   - Optimize image sizes

3. **Testing stages**:
   - Run linting and formatting checks
   - Execute unit tests
   - Perform integration tests
   - Run security scans (container images, dependencies)
   - Validate Kubernetes manifests

4. **Deployment automation**:
   - Use GitOps principles where applicable
   - Implement progressive delivery (canary, blue-green)
   - Add deployment gates and approvals
   - Configure environment-specific variables
   - Test with `helm upgrade --install --dry-run`

5. **Pipeline maintenance**:
   - Monitor pipeline success rates
   - Optimize slow jobs
   - Keep dependencies updated
   - Document pipeline behavior
   - Provide clear failure messages
