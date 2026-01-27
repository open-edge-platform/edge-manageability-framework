#!/bin/bash
set -e

APP_NAME="root-app"
NAMESPACE="onprem"
SLEEP_TIME=10

LOG_FILE="onprem_install_$(date '+%Y%m%d_%H%M%S').log"

START_TIME=$(date +%s)
PRE_END_TIME=""

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

log "🚀 Starting onprem installation"
log "📄 Log file: $LOG_FILE"

# Run installer and capture logs
./onprem_installer.sh -- --yes | while IFS= read -r line; do
  log "$line"

  if [[ "$line" == *"Pre-install completed successfully!"* ]] && [[ -z "$PRE_END_TIME" ]]; then
    PRE_END_TIME=$(date +%s)
    log "📍 Pre-install phase completed"
  fi
done

# Fallback if marker not found
if [[ -z "$PRE_END_TIME" ]]; then
  PRE_END_TIME=$(date +%s)
  log "⚠️ Pre-install marker not found, using installer end time"
fi

log "⏳ Waiting for ArgoCD application '$APP_NAME' to be Synced and Healthy..."

while true; do
  SYNC_STATUS=$(kubectl get application "$APP_NAME" -n "$NAMESPACE" \
    -o jsonpath='{.status.sync.status}' 2>/dev/null)

  HEALTH_STATUS=$(kubectl get application "$APP_NAME" -n "$NAMESPACE" \
    -o jsonpath='{.status.health.status}' 2>/dev/null)

  if [[ "$SYNC_STATUS" == "Synced" && "$HEALTH_STATUS" == "Healthy" ]]; then
    END_TIME=$(date +%s)

    PRE_TIME=$((PRE_END_TIME - START_TIME))
    POST_TIME=$((END_TIME - PRE_END_TIME))
    TOTAL_TIME=$((END_TIME - START_TIME))

    log "✅ Application '$APP_NAME' is Synced and Healthy"
    log "⏱ Pre-install time : ${PRE_TIME}s ($(date -u -d @${PRE_TIME} +%H:%M:%S))"
    log "⏱ Post-install time: ${POST_TIME}s ($(date -u -d @${POST_TIME} +%H:%M:%S))"
    log "⏱ Total time       : ${TOTAL_TIME}s ($(date -u -d @${TOTAL_TIME} +%H:%M:%S))"

    exit 0
  fi

  log "⏱ Status → Sync: ${SYNC_STATUS:-N/A}, Health: ${HEALTH_STATUS:-N/A}"
  sleep "$SLEEP_TIME"
done
