#!/bin/bash
# SPDX-FileCopyrightText: 2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0
# EMF Deployment Status — shows release pass/fail status.
# Usage:
#   ./watch-deploy.sh              # Info mode: release status only
#   ./watch-deploy.sh --debug      # Debug mode: includes pods/jobs per release
set -o pipefail
MODE="${1:-info}"
[[ "$MODE" == "--debug" ]] && MODE="debug"
HELMFILE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Source environment
if [[ -f "$HELMFILE_DIR/post-orch.env" ]]; then
  set -a; source "$HELMFILE_DIR/post-orch.env"; set +a
fi
HELMFILE_ENV="${EMF_HELMFILE_ENV:-onprem-eim}"
# Get enabled releases
releases=$(cd "$HELMFILE_DIR" && helmfile -e "$HELMFILE_ENV" list 2>/dev/null \
  | awk 'NR>1 && $3=="true" {print $1}' | sort)
total=$(echo "$releases" | wc -l)
# Build lookup from helm list
declare -A status_map ns_map
while IFS=$'\t' read -r name ns _ _ status _; do
  name=$(echo "$name" | xargs)
  status=$(echo "$status" | xargs)
  ns=$(echo "$ns" | xargs)
  [[ -z "$name" ]] && continue
  status_map["$name"]="$status"
  ns_map["$name"]="$ns"
done < <(helm list -A -a --no-headers 2>/dev/null)
# Debug mode: snapshot pods and jobs once
if [[ "$MODE" == "debug" ]]; then
  pod_snapshot=$(kubectl get pods -A --no-headers 2>/dev/null)
  job_snapshot=$(kubectl get jobs -A --no-headers 2>/dev/null)
fi
deployed=0 failed=0 pending=0 queued=0
deployed_lines="" failed_lines="" pending_lines="" queued_lines=""
while read -r name; do
  [[ -z "$name" ]] && continue
  st="${status_map[$name]:-}"
  ns="${ns_map[$name]:-}"
  # Debug: collect pod/job info
  wl=""
  if [[ "$MODE" == "debug" && -n "$ns" ]]; then
    while read -r pname ready pstatus; do
      [[ -z "$pname" ]] && continue
      case "$pstatus" in
        Running)
          if [[ "$ready" =~ ^([0-9]+)/([0-9]+)$ && "${BASH_REMATCH[1]}" == "${BASH_REMATCH[2]}" ]]; then
            wl+="      ✅ pod/$pname ($ready)"$'\n'
          else
            wl+="      ⏳ pod/$pname ($ready)"$'\n'
          fi ;;
        Completed) wl+="      ✅ pod/$pname (done)"$'\n' ;;
        *)         wl+="      ❌ pod/$pname ($pstatus)"$'\n' ;;
      esac
    done < <(echo "$pod_snapshot" | awk -v ns="$ns" -v r="$name" '$1==ns && $2 ~ "^"r {print $2, $3, $4}')
    while read -r jname completions _; do
      [[ -z "$jname" ]] && continue
      if [[ "$completions" == "1/1" ]]; then
        wl+="      ✅ job/$jname (complete)"$'\n'
      else
        wl+="      ⏳ job/$jname ($completions)"$'\n'
      fi
    done < <(echo "$job_snapshot" | awk -v ns="$ns" -v r="$name" '$1==ns && $2 ~ "^"r {print $2, $3, $4}')
  fi
  case "$st" in
    deployed)
      ((deployed++))
      deployed_lines+="  ✅  $name ($ns)"$'\n'
      [[ -n "$wl" ]] && deployed_lines+="$wl"
      ;;
    failed)
      ((failed++))
      failed_lines+="  ❌  $name ($ns)"$'\n'
      [[ -n "$wl" ]] && failed_lines+="$wl"
      ;;
    pending-*)
      ((pending++))
      pending_lines+="  ⏳  $name ($ns) [$st]"$'\n'
      [[ -n "$wl" ]] && pending_lines+="$wl"
      ;;
    *)
      ((queued++))
      queued_lines+="  ⏳  $name"$'\n'
      ;;
  esac
done <<< "$releases"
done_count=$(( deployed + failed ))
pct=0
(( total > 0 )) && pct=$(( done_count * 100 / total ))
echo ""
echo "  EMF Deployment Status  ($HELMFILE_ENV)  [$MODE]"
echo "  ════════════════════════════════════════════════"
printf "  Progress: %d/%d (%d%%)   ✅ %d  ❌ %d  ⏳ %d\n" \
  "$done_count" "$total" "$pct" "$deployed" "$failed" $(( pending + queued ))
echo "  ════════════════════════════════════════════════"
if [[ -n "$failed_lines" ]]; then
  echo ""
  echo "  FAILED:"
  echo -n "$failed_lines"
fi
if [[ -n "$pending_lines" ]]; then
  echo ""
  echo "  IN-FLIGHT:"
  echo -n "$pending_lines"
fi
if [[ -n "$queued_lines" ]]; then
  echo ""
  echo "  QUEUED:"
  echo -n "$queued_lines"
fi
if [[ -n "$deployed_lines" ]]; then
  echo ""
  echo "  DEPLOYED:"
  echo -n "$deployed_lines"
fi
echo ""
if (( deployed == total && failed == 0 )); then
  echo "  ✅  ALL $total RELEASES DEPLOYED"
elif (( done_count == total )); then
  echo "  ⚠️   DONE WITH $failed FAILURE(S)"
fi
echo ""
