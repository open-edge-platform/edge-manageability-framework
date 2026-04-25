---
name: storage-manage
description: Manage persistent storage and volumes
---

You are managing storage. Follow these steps:

1. **Storage inventory**:
   - List PVs and PVCs: `kubectl get pv,pvc -A`
   - Check StorageClasses: `kubectl get storageclass`
   - Identify unbound or orphaned volumes
   - Review storage capacity and usage

2. **Storage provisioning**:
   - Configure appropriate StorageClass
   - Set reclaim policy (Retain/Delete)
   - Define access modes correctly (RWO/ROX/RWX)
   - Set storage requests appropriately
   - Review OpenEBS LocalPV configuration

3. **Performance optimization**:
   - Choose optimal storage backend
   - Configure IOPS and throughput
   - Use local storage for high-performance needs
   - Review volume mount options
   - Check for I/O bottlenecks

4. **Data management**:
   - Implement backup strategies for PVs
   - Plan volume expansion procedures
   - Document data retention policies
   - Configure snapshot schedules
   - Test restore procedures

5. **Troubleshooting**:
   - Debug mount issues: `kubectl describe pod <pod>`
   - Check PVC events
   - Verify StorageClass provisioner
   - Review CSI driver logs
   - Provide resolution steps for common issues
