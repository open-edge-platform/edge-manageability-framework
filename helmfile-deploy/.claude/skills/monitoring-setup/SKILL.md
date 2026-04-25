---
name: monitoring-setup
description: Set up comprehensive monitoring and alerting
---

You are setting up monitoring. Follow these steps:

1. **Metrics collection**:
   - Deploy Prometheus/metrics server
   - Configure ServiceMonitors for applications
   - Set up custom metrics exporters
   - Verify metric scraping: `kubectl get servicemonitors -A`

2. **Visualization**:
   - Create Grafana dashboards
   - Design dashboards for: cluster health, application metrics, resource usage
   - Use meaningful labels and annotations
   - Set up dashboard variables for filtering

3. **Alerting rules**:
   - Define SLIs and SLOs
   - Create PrometheusRules for critical alerts
   - Set appropriate thresholds
   - Avoid alert fatigue with proper grouping
   - Test alert firing

4. **Log aggregation**:
   - Configure log collection (Loki, ELK, etc.)
   - Set up log retention policies
   - Create useful log queries
   - Configure log-based alerts

5. **Alert routing**:
   - Configure AlertManager receivers
   - Set up notification channels (Slack, PagerDuty, email)
   - Define escalation policies
   - Create runbooks for common alerts
   - Document on-call procedures
