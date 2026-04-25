---
name: dashboard
description: Create comprehensive status dashboard for edge manageability framework
---

You are creating a status dashboard. Follow these steps:

1. **Gather cluster metrics**:
   - Node status and resource utilization
   - Pod health across all namespaces
   - Helm release status
   - PVC and storage usage
   - Recent events and alerts

2. **Component status check**:
   - Pre-orchestration services (OpenEBS LocalPV, MetalLB)
   - Post-orchestration services (Istio, Kyverno, PostgreSQL operator)
   - External Secrets status
   - Certificate manager health
   - Ingress gateway status

3. **Generate HTML dashboard**:
   - Create a responsive HTML file with:
     - Overall health indicator (green/yellow/red)
     - Cluster overview section (nodes, pods, namespaces)
     - Helm releases table with status
     - Resource utilization charts (using inline SVG or Chart.js)
     - Recent events timeline
     - Quick action buttons for common operations
   - Use modern CSS framework (Tailwind or Bootstrap)
   - Add auto-refresh functionality
   - Include timestamp of last update

4. **Add interactive features**:
   - Filter by namespace or component
   - Search functionality
   - Expandable sections for details
   - Export data option
   - Links to logs and metrics

5. **Deployment options**:
   - Save dashboard to `/tmp/emf-dashboard.html`
   - Optionally deploy as Kubernetes service
   - Set up dashboard refresh cron job
   - Provide URL or instructions to access
   - Document dashboard maintenance

**Output format**: Generate a complete, standalone HTML file with embedded CSS and JavaScript that provides a real-time view of the edge manageability framework health.
