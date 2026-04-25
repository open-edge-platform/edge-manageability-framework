# Edge Manageability Framework - Claude Skills

This directory contains 20 custom Claude skills designed specifically for managing the edge manageability framework helmfile deployment.

## Available Skills

### Deployment & Operations
1. **helm-deploy** - Deploy Helm charts with validation and health checks
2. **helm-rollback** - Safely rollback a Helm release to a previous revision
3. **helmfile-sync** - Synchronize Helmfile deployments across environments

### Debugging & Troubleshooting
4. **k8s-debug** - Debug Kubernetes pods and services systematically
5. **health-check** - Comprehensive cluster and application health check
6. **performance-check** - Analyze cluster and application performance metrics

### Security & Compliance
7. **secret-audit** - Audit secrets management and detect exposed credentials
8. **pod-security** - Enforce pod security standards and policies
9. **rbac-audit** - Audit and optimize Kubernetes RBAC permissions
10. **network-policy** - Create and validate Kubernetes network policies

### Infrastructure Management
11. **istio-check** - Validate Istio service mesh configuration and health
12. **cert-manager** - Manage TLS certificates with cert-manager
13. **storage-manage** - Manage persistent storage and volumes
14. **monitoring-setup** - Set up comprehensive monitoring and alerting

### Validation & Best Practices
15. **yaml-validate** - Validate YAML files for syntax and Kubernetes schema compliance
16. **disaster-recovery** - Create and test disaster recovery procedures
17. **cost-analysis** - Analyze Kubernetes cluster costs and optimization opportunities

### Development Workflow
18. **git-flow** - Manage git workflow with best practices for this project
19. **ci-cd-pipeline** - Build and optimize CI/CD pipelines

### Visualization
20. **dashboard** - Create comprehensive status dashboard for edge manageability framework

## Usage

To use any skill, invoke it with the `/` prefix in Claude Code:

```bash
/helm-deploy
/k8s-debug
/dashboard
```

## Skills Location

All skills are self-contained within the `helmfile-deploy/.claude/skills/` directory, making this folder portable and shareable.

## Customization

Each skill is a markdown file with:
- Frontmatter (name, description)
- Step-by-step instructions
- Best practices specific to this project

Feel free to modify any skill to match your specific requirements or add new skills following the same format.
