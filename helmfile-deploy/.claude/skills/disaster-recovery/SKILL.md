---
name: disaster-recovery
description: Create and test disaster recovery procedures
---

You are working on disaster recovery. Follow these steps:

1. **Backup assessment**:
   - List all critical resources to backup
   - Check existing backup solutions (Velero, etcd backups)
   - Verify backup schedules and retention
   - Test backup accessibility

2. **Create backup plan**:
   - Document which resources to backup (PVs, configs, secrets, CRDs)
   - Set up automated backup jobs
   - Configure backup storage location
   - Test backup creation

3. **Restore procedure**:
   - Document step-by-step restore process
   - Create restore scripts/commands
   - Define RTO (Recovery Time Objective) and RPO (Recovery Point Objective)
   - Identify dependencies and restore order

4. **Test disaster scenarios**:
   - Simulate namespace deletion
   - Test PV restore
   - Validate application recovery
   - Verify data integrity after restore
   - Document recovery time

5. **Documentation**:
   - Create runbook with clear procedures
   - List emergency contacts
   - Document known issues and workarounds
   - Include rollback procedures
   - Keep documentation updated and accessible
