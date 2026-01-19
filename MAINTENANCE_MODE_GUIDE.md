# Traefik Maintenance Mode Configuration Guide

## Overview

This guide explains how to configure and test the maintenance mode feature for Traefik in the Edge Manageability Framework using the official [Traefik Maintenance Plugin](https://plugins.traefik.io/plugins/637ba08fc672f04dd500d1a1/maintenance-page).

## How It Works

The maintenance mode uses the `traefik-maintenance` plugin which:
- Checks if a trigger file exists in the filesystem
- If the trigger file exists, returns the maintenance page
- Uses a ConfigMap to store the maintenance HTML page
- Returns HTTP 503 (Service Unavailable) during maintenance

## Files Modified

### 1. Traefik Static Configuration
**File**: [argocd/applications/configs/traefik.yaml](argocd/applications/configs/traefik.yaml)

This file contains:
- Plugin configuration: `traefik-maintenance` plugin from `github.com/TRIMM/traefik-maintenance`
- Volume mounts: `maintenance-page` ConfigMap and `maintenance-trigger` emptyDir

### 2. Traefik Extra Objects Configuration
**File**: [argocd/applications/configs/traefik-extra-objects.yaml](argocd/applications/configs/traefik-extra-objects.yaml)

This file contains:
- `maintenanceMode.enabled`: Enable/disable the maintenance middleware
- `maintenanceMode.triggerFilename`: Path to the trigger file
- `maintenanceMode.maintenanceFilename`: Path to the maintenance HTML
- `maintenanceMode.httpStatusCode`: HTTP status (default: 503)
- `maintenancePageContent`: HTML content for the maintenance page

### 3. Custom Template File
**File**: [argocd/applications/custom/traefik-extra-objects.tpl](argocd/applications/custom/traefik-extra-objects.tpl)

This template file allows values-based configuration of maintenance mode through ArgoCD values.

## How to Enable Maintenance Mode

**Prerequisites**: Ensure the traefik-extra-objects application is synced first.

### Step 1: Sync ArgoCD Applications

```bash
# Sync Traefik with the new plugin configuration
argocd app sync traefik

# Sync traefik-extra-objects with maintenance middleware
argocd app sync traefik-extra-objects

# Or via kubectl
kubectl -n argocd patch application traefik -p '{"operation":{"sync":{}}}' --type merge
kubectl -n argocd patch application traefik-extra-objects -p '{"operation":{"sync":{}}}' --type merge
```

### Step 2: Create the Trigger File (Activate Maintenance)

```bash
# Enable maintenance mode by creating the trigger file
kubectl exec -n orch-gateway deploy/traefik -- touch /var/run/maintenance/maintenance.trigger

# Verify the file was created
kubectl exec -n orch-gateway deploy/traefik -- ls -la /var/run/maintenance/
```

### Step 3: Disable Maintenance Mode

```bash
# Remove the trigger file to disable maintenance mode
kubectl exec -n orch-gateway deploy/traefik -- rm /var/run/maintenance/maintenance.trigger
```

### Configuration via Values (Advanced)

Add to your values file (e.g., in `orch-configs/profiles/`):

```yaml
argo:
  traefik:
    maintenanceMode:
      enabled: true
      triggerFilename: "/var/run/maintenance/maintenance.trigger"
      maintenanceFilename: "/maintenance/maintenance.html"
      httpStatusCode: 503
```

## Customizing the Maintenance Page

Edit the `maintenancePageContent` section in [argocd/applications/configs/traefik-extra-objects.yaml](argocd/applications/configs/traefik-extra-objects.yaml):

```yaml
maintenancePageContent: |
  <!DOCTYPE html>
  <html lang="en">
  <head>
      <meta charset="UTF-8">
      <title>System Maintenance</title>
      <!-- Customize your HTML here -->
  </head>
  <body>
      <h1>Custom Maintenance Message</h1>
      <p>Expected downtime: 2 hours</p>
  </body>
  </html>
```

## Testing the Maintenance Mode

### Step 1: Verify Plugin Installation

Check that Traefik loaded the maintenance plugin:

```bash
# Check Traefik logs for plugin loading
kubectl -n orch-gateway logs -l app.kubernetes.io/name=traefik | grep -i maintenance

# Check running pods
kubectl -n orch-gateway get pods -l app.kubernetes.io/name=traefik
```

### Step 2: Verify the ConfigMap

Check that the maintenance page ConfigMap was created:

```bash
kubectl -n orch-gateway get configmap | grep maintenance
kubectl -n orch-gateway describe configmap traefik-maintenance-page
```

### Step 3: Verify the Middleware

Check that the maintenance middleware was created:

```bash
kubectl -n orch-gateway get middleware
kubectl -n orch-gateway describe middleware maintenance-mode
```

### Step 4: Test Maintenance Mode Activation

**Before Activation** (should work normally):
```bash
curl -k https://your-domain.com -w "\nHTTP Status: %{http_code}\n"
```

**Activate Maintenance Mode**:
```bash
kubectl exec -n orch-gateway deploy/traefik -- touch /var/run/maintenance/maintenance.trigger
```

**After Activation** (should return 503 with maintenance page):
```bash
curl -k https://your-domain.com -w "\nHTTP Status: %{http_code}\n"
```

Expected output:
- HTTP Status: 503
- Body: HTML maintenance page

### Step 5: Verify Maintenance Page Content

```bash
# Get the full maintenance page response
curl -k https://your-domain.com

# Check if it's the maintenance HTML
curl -k https://your-domain.com | grep "System Maintenance"
```

### Step 6: Test Deactivation

**Deactivate Maintenance Mode**:
```bash
kubectl exec -n orch-gateway deploy/traefik -- rm /var/run/maintenance/maintenance.trigger
```

**Verify Normal Operation**:
```bash
curl -k https://your-domain.com -w "\nHTTP Status: %{http_code}\n"
```

### Step 7: Check Traefik Logs

Monitor Traefik logs to see maintenance plugin in action:

```bash
kubectl -n orch-gateway logs -l app.kubernetes.io/name=traefik -f
```

## How the Maintenance Mode Works

The system uses the official Traefik Maintenance Plugin:

1. **Plugin Installation**: Traefik loads the `traefik-maintenance` plugin from GitHub
2. **ConfigMap Creation**: Creates a ConfigMap with the maintenance page HTML mounted at `/maintenance/maintenance.html`
3. **Trigger File**: Uses an emptyDir volume at `/var/run/maintenance/` for the trigger file
4. **Middleware Creation**: Creates a Traefik Middleware that:
   - Checks if `/var/run/maintenance/maintenance.trigger` exists
   - If exists: Returns the maintenance page with HTTP 503
   - If not exists: Allows traffic through normally
5. **Router Integration**: The middleware can be attached to IngressRoutes as needed

## Troubleshooting

### Maintenance Page Not Showing

1. **Check if the trigger file exists**:
   ```bash
   kubectl exec -n orch-gateway deploy/traefik -- ls -la /var/run/maintenance/
   ```

2. **Check if the middleware was created**:
   ```bash
   kubectl -n orch-gateway get middleware maintenance-mode
   kubectl -n orch-gateway describe middleware maintenance-mode
   ```

3. **Verify the ConfigMap is mounted**:
   ```bash
   kubectl exec -n orch-gateway deploy/traefik -- cat /maintenance/maintenance.html
   ```

4. **Check if IngressRoutes are using the middleware**:
   ```bash
   kubectl -n orch-gateway get ingressroute -o yaml | grep maintenance
   ```

5. **Check Traefik dashboard** (if enabled):
   ```bash
   kubectl -n orch-gateway port-forward svc/traefik 9000:9000
   # Visit http://localhost:9000/dashboard/
   ```

### Plugin Not Loading

1. **Check Traefik pod logs**:
   ```bash
   kubectl -n orch-gateway logs -l app.kubernetes.io/name=traefik | grep -i "plugin\|experimental"
   ```

2. **Verify plugin arguments**:
   ```bash
   kubectl -n orch-gateway get deployment traefik -o yaml | grep experimental
   ```

3. **Check if pod restarted after config change**:
   ```bash
   kubectl -n orch-gateway rollout status deployment traefik
   kubectl -n orch-gateway rollout restart deployment traefik
   ```

### Trigger File Disappears After Pod Restart

This is expected! The trigger file is in an emptyDir volume which is ephemeral. Solutions:

1. **Use a persistent script** to create the trigger on startup
2. **Use a ConfigMap** to control the trigger (requires middleware restart)
3. **Create a helper script** for maintenance mode toggling

## Disabling Maintenance Mode

Simply remove the trigger file:

```bash
# Quick disable - remove the trigger file
kubectl exec -n orch-gateway deploy/traefik -- rm -f /var/run/maintenance/maintenance.trigger

# Verify it's removed
kubectl exec -n orch-gateway deploy/traefik -- ls -la /var/run/maintenance/
```

To completely disable the middleware (optional):

```bash
# Edit the config and set enabled: false
kubectl -n orch-gateway edit middleware maintenance-mode

# Or update via ArgoCD values and sync
# Set enabled: false in your values file
```

## Production Considerations

1. **Schedule Maintenance Windows**: Plan and communicate maintenance windows in advance
2. **Automation**: Create scripts or automation to toggle maintenance mode
3. **Monitoring**: Set up alerts for HTTP 503 responses or maintenance mode activation
4. **Documentation**: Update your runbooks with this procedure
5. **Testing**: Test maintenance mode in a non-production environment first
6. **Persistent Trigger**: Consider using a persistent volume or init container for the trigger file
7. **Multiple Instances**: If running multiple Traefik replicas, ensure the trigger file is shared (use PVC)

## Quick Start: Complete Setup Example

### 1. Deploy the Configuration

The configuration is already set up in:
- [argocd/applications/configs/traefik.yaml](argocd/applications/configs/traefik.yaml) - Plugin configured
- [argocd/applications/configs/traefik-extra-objects.yaml](argocd/applications/configs/traefik-extra-objects.yaml) - Maintenance page

### 2. Sync ArgoCD Applications

```bash
# Sync Traefik with plugin
argocd app sync traefik
argocd app sync traefik-extra-objects
```

### 3. Enable Maintenance Mode

```bash
# Activate maintenance mode
kubectl exec -n orch-gateway deploy/traefik -- touch /var/run/maintenance/maintenance.trigger
```

### 4. Test It

```bash
# Should return HTTP 503 with maintenance page
curl -k https://your-cluster-domain.com
```

### 5. Disable Maintenance Mode

```bash
# Deactivate maintenance mode
kubectl exec -n orch-gateway deploy/traefik -- rm /var/run/maintenance/maintenance.trigger
```

## Helper Script

Create a helper script to easily toggle maintenance mode:

```bash
#!/bin/bash
# maintenance-toggle.sh

NAMESPACE="orch-gateway"
DEPLOYMENT="traefik"
TRIGGER_FILE="/var/run/maintenance/maintenance.trigger"

case "$1" in
  on|enable)
    echo "Enabling maintenance mode..."
    kubectl exec -n $NAMESPACE deploy/$DEPLOYMENT -- touch $TRIGGER_FILE
    echo "✓ Maintenance mode enabled"
    ;;
  off|disable)
    echo "Disabling maintenance mode..."
    kubectl exec -n $NAMESPACE deploy/$DEPLOYMENT -- rm -f $TRIGGER_FILE
    echo "✓ Maintenance mode disabled"
    ;;
  status)
    if kubectl exec -n $NAMESPACE deploy/$DEPLOYMENT -- test -f $TRIGGER_FILE 2>/dev/null; then
      echo "⚠ Maintenance mode is ENABLED"
      exit 1
    else
      echo "✓ Maintenance mode is DISABLED"
      exit 0
    fi
    ;;
  *)
    echo "Usage: $0 {on|off|status}"
    exit 1
    ;;
esac
```

Usage:
```bash
chmod +x maintenance-toggle.sh
./maintenance-toggle.sh on      # Enable
./maintenance-toggle.sh status  # Check
./maintenance-toggle.sh off     # Disable
```

## Notes

- The maintenance mode feature uses the official Traefik plugin: https://plugins.traefik.io/plugins/637ba08fc672f04dd500d1a1/maintenance-page
- The trigger file is stored in an emptyDir volume, so it's ephemeral (resets on pod restart)
- For production use, consider using a persistent volume or automation for the trigger file
- The plugin checks the trigger file on every request, so there's no caching delay
- You can serve JSON responses by changing `maintenanceFilename` to point to a JSON file and setting `httpContentType` to `application/json`

## Reference Links

- Plugin Documentation: https://plugins.traefik.io/plugins/637ba08fc672f04dd500d1a1/maintenance-page
- Plugin Source: https://github.com/TRIMM/traefik-maintenance
- Traefik Plugins Guide: https://doc.traefik.io/traefik/plugins/
