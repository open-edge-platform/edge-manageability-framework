#!/usr/bin/env python3
# SPDX-FileCopyrightText: (C) 2026 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

"""
Diagnostics Convergence Analyzer
=================================

Analyzes diagnostics artifacts across multiple workflow runs to identify:
- Error patterns that persist across runs
- Transient vs recurring issues
- Convergence trends (are errors decreasing/increasing?)
- Most frequent issue types
- Likely blockers / dependency chains (from ArgoCD status messages)

Usage:
    python ci/diagnostics_convergence.py <artifacts_dir>

Where artifacts_dir contains extracted diagnostics artifacts with diagnostics_full_*.json files.

Outputs:
- convergence.json: Full convergence analysis with error trends + issue-type and blocker aggregates
- convergence.md: Human-readable markdown report
- manifest.json: List of analyzed artifacts with metadata
"""

import argparse
import hashlib
import json
import os
import re
import sys
from collections import Counter, defaultdict
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

# Convergence analysis constants
PERSISTENT_ERROR_THRESHOLD = 0.5  # Errors occurring in >50% of runs are considered persistent
IMPROVING_THRESHOLD = 0.8  # Trend is improving if errors decrease by >20%
DEGRADING_THRESHOLD = 1.2  # Trend is degrading if errors increase by >20%

# Regexes for ArgoCD causal blockers
ARGOCD_WAIT_DEPLOY_RE = re.compile(r"waiting for healthy state of apps/Deployment/([a-zA-Z0-9-_.]+)")
ARGOCD_WAIT_APP_RE = re.compile(r"waiting for healthy state of argoproj\.io/Application/([a-zA-Z0-9-_.]+)")

# Heuristic to turn pod name into workload: strip "-<rs-hash>-<pod-suffix>"
# Example: dm-manager-84987656cf-kvn52 -> dm-manager
POD_WORKLOAD_RE = re.compile(r"^(?P<base>.+)-[a-f0-9]{8,10}-[a-z0-9]{4,6}$")


def _short_hash(text: str, length: int = 8) -> str:
    if not text:
        return "nohash"
    return hashlib.sha1(text.encode("utf-8", errors="ignore")).hexdigest()[:length]


def _normalize_text(text: str) -> str:
    """Normalize noisy text by removing GUID-like tokens and collapsing whitespace."""
    if not text:
        return ""
    t = text
    t = re.sub(r"\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b", "<uuid>", t, flags=re.IGNORECASE)
    t = re.sub(r"\s+", " ", t).strip()
    return t


def normalize_pod_to_workload(pod_name: str) -> str:
    m = POD_WORKLOAD_RE.match(pod_name or "")
    if m:
        return m.group("base")
    # Fallback: strip last 2 segments if they look like rs-hash + suffix
    parts = (pod_name or "").split("-")
    if len(parts) >= 3 and re.fullmatch(r"[a-f0-9]{8,10}", parts[-2]) and re.fullmatch(r"[a-z0-9]{4,6}", parts[-1]):
        return "-".join(parts[:-2])
    return pod_name or "unknown"


def parse_job_from_artifact_dir(artifact_dirname: str) -> Dict[str, str]:
    """
    artifact_dirname example: diagnostics-deploy-on-prem-pull_request-1458-1
    Return:
      - job: deploy-on-prem
      - event: pull_request
      - pr_or_run: 1458 (best effort)
      - attempt: 1 (best effort)
    """
    out = {"job": "unknown", "event": "unknown", "pr_or_run": "", "attempt": ""}
    name = artifact_dirname or ""
    if not name.startswith("diagnostics-"):
        return out

    # Find an event marker that separates job from the rest
    for event in ["pull_request", "push", "merge_group", "workflow_dispatch", "schedule"]:
        marker = f"-{event}-"
        if marker in name:
            before, after = name.split(marker, 1)
            out["job"] = before[len("diagnostics-") :]
            out["event"] = event
            # remaining often looks like "<num>-<attempt>"
            m = re.match(r"(?P<num>\d+)(?:-(?P<attempt>\d+))?", after)
            if m:
                out["pr_or_run"] = m.group("num") or ""
                out["attempt"] = m.group("attempt") or ""
            return out

    # Fallback: old format
    out["job"] = name[len("diagnostics-") :]
    return out


def deployment_issue_type(dep: Dict[str, Any]) -> str:
    """
    Prefer a non-ready condition reason (Available=False or Progressing=False) over dep['reason'].
    """
    conditions = dep.get("conditions") or []
    # Prefer Available=False reason
    for c in conditions:
        if c.get("type") == "Available" and str(c.get("status")) == "False":
            return c.get("reason") or "AvailableFalse"
    # Then Progressing=False
    for c in conditions:
        if c.get("type") == "Progressing" and str(c.get("status")) == "False":
            return c.get("reason") or "ProgressingFalse"
    # Fallback to top-level reason
    return dep.get("reason") or "DeploymentNotReady"


def argocd_issue_type(app: Dict[str, Any]) -> str:
    health = app.get("health") or "UnknownHealth"
    sync = app.get("sync") or "UnknownSync"
    phase = app.get("operation_phase") or "UnknownPhase"
    return f"{health}::{sync}::{phase}"


def extract_blockers_from_argocd_message(message: str) -> List[str]:
    blockers: List[str] = []
    if not message:
        return blockers
    for m in ARGOCD_WAIT_DEPLOY_RE.finditer(message):
        blockers.append(f"Deployment/{m.group(1)}")
    for m in ARGOCD_WAIT_APP_RE.finditer(message):
        blockers.append(f"Application/{m.group(1)}")
    return blockers


def extract_issue_records(diag: Dict[str, Any]) -> List[Dict[str, Any]]:
    """
    Convert diagnostics summary into a list of normalized issue records:
      {
        domain: "pod" | "deploy" | "argocd" | "pvc",
        component: "...",
        issue_type: "...",
        signature: "...",
        cause_hints: [...],
        blockers: [...],   # only for argocd right now
        raw: {...}
      }
    """
    summary = (diag or {}).get("summary", {}) or {}
    issues: List[Dict[str, Any]] = []

    # Pods with errors
    for e in summary.get("pods_w_errors", []) or []:
        ns = e.get("namespace") or "unknown"
        pod = e.get("pod") or e.get("name") or "unknown"
        workload = normalize_pod_to_workload(pod)
        itype = e.get("reason") or e.get("status") or "PodError"
        msg = _normalize_text(e.get("message") or "")
        last_event = _normalize_text(e.get("last_event") or "")
        hint_bits = []
        if itype:
            hint_bits.append(itype)
        if e.get("restart_count") is not None:
            hint_bits.append(f"restarts={e.get('restart_count')}")
        if msg:
            hint_bits.append(f"msg={msg[:160]}")
        if last_event:
            hint_bits.append(f"event={last_event[:160]}")

        detail_hash = _short_hash(msg or last_event)
        component = f"pod::{ns}::{workload}"
        signature = f"{component}::{itype}::{detail_hash}"

        issues.append(
            {
                "domain": "pod",
                "component": component,
                "issue_type": itype,
                "signature": signature,
                "cause_hints": hint_bits,
                "blockers": [],
                "raw": e,
            }
        )

    # Deployments not ready
    for d in summary.get("deployments_not_ready", []) or []:
        ns = d.get("namespace") or "unknown"
        name = d.get("name") or d.get("deployment") or "unknown"
        itype = deployment_issue_type(d)
        component = f"deploy::{ns}::{name}"
        signature = f"{component}::{itype}"

        hint_bits = [itype]
        # Include condition message (normalized) if present for debugging (hashed into signature would be too noisy)
        conditions = d.get("conditions") or []
        for c in conditions:
            if c.get("type") == "Available" and str(c.get("status")) == "False":
                cm = _normalize_text(c.get("message") or "")
                if cm:
                    hint_bits.append(f"available_msg={cm[:160]}")
                break

        issues.append(
            {
                "domain": "deploy",
                "component": component,
                "issue_type": itype,
                "signature": signature,
                "cause_hints": hint_bits,
                "blockers": [],
                "raw": d,
            }
        )

    # PVC not bound
    for p in summary.get("pvc_not_bound", []) or []:
        ns = p.get("namespace") or "unknown"
        name = p.get("name") or p.get("pvc") or "unknown"
        itype = p.get("reason") or p.get("status") or "PVCNotBound"
        component = f"pvc::{ns}::{name}"
        signature = f"{component}::{itype}"
        hint_bits = [itype]
        msg = _normalize_text(p.get("message") or "")
        if msg:
            hint_bits.append(f"msg={msg[:160]}")
        issues.append(
            {
                "domain": "pvc",
                "component": component,
                "issue_type": itype,
                "signature": signature,
                "cause_hints": hint_bits,
                "blockers": [],
                "raw": p,
            }
        )

    # ArgoCD unhealthy
    for a in summary.get("argocd_apps_unhealthy", []) or []:
        ns = a.get("namespace") or "unknown"
        name = a.get("name") or "unknown"
        itype = argocd_issue_type(a)
        msg = _normalize_text(a.get("message") or "")
        detail_hash = _short_hash(msg)
        component = f"argocd::{ns}::{name}"
        signature = f"{component}::{itype}::{detail_hash}"

        blockers = extract_blockers_from_argocd_message(a.get("message") or "")
        hint_bits = [itype]
        if a.get("out_of_sync_count") is not None:
            hint_bits.append(f"out_of_sync_count={a.get('out_of_sync_count')}")
        oos = a.get("out_of_sync_resources") or []
        if oos:
            hint_bits.append(f"out_of_sync_examples={'; '.join(list(oos)[:3])}")
        if blockers:
            hint_bits.append(f"blockers={', '.join(blockers[:3])}")
        if msg:
            hint_bits.append(f"msg={msg[:160]}")

        issues.append(
            {
                "domain": "argocd",
                "component": component,
                "issue_type": itype,
                "signature": signature,
                "cause_hints": hint_bits,
                "blockers": blockers,
                "raw": a,
            }
        )

    return issues


def analyze_convergence(artifacts_data: List[Dict[str, Any]]) -> Dict[str, Any]:
    if not artifacts_data:
        return {
            "status": "no_data",
            "message": "No diagnostics data available for analysis",
            "total_runs": 0,
        }

    artifacts_data.sort(key=lambda x: x.get("timestamp", ""))

    error_occurrences: Dict[str, List[Dict[str, Any]]] = defaultdict(list)
    issue_type_counts = Counter()
    component_counts = Counter()
    blocker_counts = Counter()
    issue_type_by_component = defaultdict(Counter)

    run_summaries: List[Dict[str, Any]] = []

    for idx, artifact in enumerate(artifacts_data):
        run_num = idx + 1
        diag = artifact.get("diagnostics", {})
        issues = extract_issue_records(diag)

        # Summary counts per domain
        domain_counts = Counter([i["domain"] for i in issues])
        total_errors = len(issues)

        run_summaries.append(
            {
                "run": run_num,
                "timestamp": artifact.get("timestamp"),
                "workflow_run_id": artifact.get("workflow_run_id"),
                "job_name": artifact.get("job_name"),
                "job": artifact.get("job"),
                "event": artifact.get("event"),
                "total_errors": total_errors,
                "pod_errors": domain_counts.get("pod", 0),
                "deployment_errors": domain_counts.get("deploy", 0),
                "argocd_errors": domain_counts.get("argocd", 0),
                "pvc_errors": domain_counts.get("pvc", 0),
            }
        )

        # Track issues
        for issue in issues:
            sig = issue["signature"]
            error_occurrences[sig].append(
                {
                    "run": run_num,
                    "timestamp": artifact.get("timestamp"),
                    "component": issue["component"],
                    "issue_type": issue["issue_type"],
                    "domain": issue["domain"],
                    "cause_hints": issue.get("cause_hints", []),
                    "blockers": issue.get("blockers", []),
                }
            )

            issue_type_counts[issue["issue_type"]] += 1
            component_counts[issue["component"]] += 1
            issue_type_by_component[issue["component"]][issue["issue_type"]] += 1
            for b in issue.get("blockers", []) or []:
                blocker_counts[b] += 1

    persistent_errors = []
    transient_errors = []

    total_runs = len(artifacts_data)
    for sig, occurrences in error_occurrences.items():
        occurrence_rate = len(occurrences) / total_runs

        # compact examples for output
        example = occurrences[0]
        error_info = {
            "signature": sig,
            "domain": example.get("domain"),
            "component": example.get("component"),
            "issue_type": example.get("issue_type"),
            "occurrences": len(occurrences),
            "occurrence_rate": occurrence_rate,
            "first_seen": occurrences[0]["timestamp"],
            "last_seen": occurrences[-1]["timestamp"],
            "runs_affected": [occ["run"] for occ in occurrences],
            "example_cause_hints": example.get("cause_hints", [])[:5],
            "example_blockers": example.get("blockers", [])[:5],
        }

        if occurrence_rate > PERSISTENT_ERROR_THRESHOLD:
            persistent_errors.append(error_info)
        else:
            transient_errors.append(error_info)

    # Trend calculation
    if len(run_summaries) >= 2:
        first_half = run_summaries[: len(run_summaries) // 2]
        second_half = run_summaries[len(run_summaries) // 2 :]

        avg_errors_first = sum(r["total_errors"] for r in first_half) / len(first_half)
        avg_errors_second = sum(r["total_errors"] for r in second_half) / len(second_half)

        if avg_errors_second < avg_errors_first * IMPROVING_THRESHOLD:
            trend = "improving"
        elif avg_errors_second > avg_errors_first * DEGRADING_THRESHOLD:
            trend = "degrading"
        else:
            trend = "stable"
    else:
        trend = "insufficient_data"
        avg_errors_first = 0.0
        avg_errors_second = 0.0

    return {
        "status": "success",
        "total_runs": total_runs,
        "run_summaries": run_summaries,
        "persistent_errors": sorted(persistent_errors, key=lambda x: x["occurrence_rate"], reverse=True),
        "transient_errors": sorted(transient_errors, key=lambda x: x["occurrences"], reverse=True),
        "top_issue_types": [{"issue_type": k, "count": v} for k, v in issue_type_counts.most_common(20)],
        "top_components": [{"component": k, "count": v} for k, v in component_counts.most_common(20)],
        "top_blockers": [{"blocker": k, "count": v} for k, v in blocker_counts.most_common(20)],
        "issue_types_by_component": {
            comp: [{"issue_type": it, "count": c} for it, c in cnt.most_common(10)]
            for comp, cnt in issue_type_by_component.items()
        },
        "convergence_trend": {
            "trend": trend,
            "avg_errors_first_half": avg_errors_first,
            "avg_errors_second_half": avg_errors_second,
        },
    }


def generate_markdown_report(convergence: Dict[str, Any]) -> str:
    md = ["# Diagnostics Convergence Analysis\n"]

    if convergence.get("status") != "success":
        md.append(f"**Status:** {convergence.get('status')}\n")
        md.append(f"{convergence.get('message', 'Analysis incomplete')}\n")
        return "\n".join(md)

    md.append(f"**Analysis Date:** {datetime.now().isoformat()}\n")
    md.append(f"**Total Runs Analyzed:** {convergence['total_runs']}\n\n")

    # Trend
    trend = convergence["convergence_trend"]
    md.append("## Convergence Trend\n")
    md.append(f"**Overall Trend:** {trend['trend'].upper()}\n")
    md.append(f"- Average issues (first half): {trend['avg_errors_first_half']:.1f}\n")
    md.append(f"- Average issues (second half): {trend['avg_errors_second_half']:.1f}\n\n")

    # NEW: Top issue types
    md.append("## Top Issue Types (Most Observed)\n")
    top_issue_types = convergence.get("top_issue_types", [])
    if top_issue_types:
        md.append("| Issue Type | Count |\n")
        md.append("|-----------|-------|\n")
        for row in top_issue_types[:15]:
            md.append(f"| {row['issue_type']} | {row['count']} |\n")
        md.append("\n")
    else:
        md.append("*No issue types detected* ✅\n\n")

    # NEW: Top blockers (ArgoCD dependency hints)
    md.append("## Top Blockers / Dependencies (from ArgoCD messages)\n")
    top_blockers = convergence.get("top_blockers", [])
    if top_blockers:
        md.append("| Blocker | Count |\n")
        md.append("|--------|-------|\n")
        for row in top_blockers[:15]:
            md.append(f"| {row['blocker']} | {row['count']} |\n")
        md.append("\n")
    else:
        md.append("*No blockers detected* ✅\n\n")

    # Persistent errors
    persistent = convergence["persistent_errors"]
    threshold_pct = int(PERSISTENT_ERROR_THRESHOLD * 100)
    md.append(f"## Persistent Patterns ({len(persistent)})\n")
    md.append(f"These signatures occur in >{threshold_pct}% of runs:\n\n")

    if persistent:
        for err in persistent[:10]:
            md.append(f"### `{err['signature']}`\n")
            md.append(f"- **Domain:** {err.get('domain')}\n")
            md.append(f"- **Component:** {err.get('component')}\n")
            md.append(f"- **Issue Type:** {err.get('issue_type')}\n")
            md.append(f"- **Occurrence Rate:** {err['occurrence_rate']*100:.1f}% ({err['occurrences']}/{convergence['total_runs']} runs)\n")
            md.append(f"- **First Seen:** {err['first_seen']}\n")
            md.append(f"- **Runs Affected:** {', '.join(map(str, err['runs_affected']))}\n")
            hints = err.get("example_cause_hints") or []
            if hints:
                md.append(f"- **Cause hints (example):** {', '.join(hints[:4])}\n")
            blockers = err.get("example_blockers") or []
            if blockers:
                md.append(f"- **Blockers (example):** {', '.join(blockers[:4])}\n")
            md.append("\n")
    else:
        md.append("*No persistent patterns detected* ✅\n\n")

    # Transient errors
    transient = convergence["transient_errors"]
    md.append(f"## Transient Patterns ({len(transient)})\n")
    md.append("These signatures occur sporadically (<50% of runs):\n\n")

    if transient:
        for err in transient[:10]:
            md.append(f"- `{err['signature']}` ({err['occurrences']}x)\n")
    else:
        md.append("*No transient patterns detected* ✅\n")
    md.append("\n")

    # Run summary
    md.append("## Run Summary\n\n")
    md.append("| Run | Timestamp | Artifact Dir | Job | Event | Total | Pod | Deploy | ArgoCD | PVC |\n")
    md.append("|-----|-----------|--------------|-----|-------|-------|-----|--------|--------|-----|\n")

    for run in convergence["run_summaries"]:
        ts = run["timestamp"][:19] if run.get("timestamp") else "N/A"
        md.append(
            f"| {run['run']} | {ts} | {run.get('job_name','')} | {run.get('job','')} | {run.get('event','')} | "
            f"{run['total_errors']} | {run['pod_errors']} | {run['deployment_errors']} | {run['argocd_errors']} | {run['pvc_errors']} |\n"
        )

    return "".join(md)


def process_artifacts(artifacts_dir: str) -> Tuple[List[Dict[str, Any]], List[Dict[str, Any]]]:
    artifacts_path = Path(artifacts_dir)
    artifacts_data: List[Dict[str, Any]] = []
    manifest: List[Dict[str, Any]] = []

    json_files = list(artifacts_path.glob("**/diagnostics_full_*.json"))
    print(f"Found {len(json_files)} diagnostics JSON files", file=sys.stderr)

    for json_file in json_files:
        try:
            with open(json_file, "r") as f:
                diag_data = json.load(f)

            relative_path = json_file.relative_to(artifacts_path)
            parts = relative_path.parts

            artifact_dir = parts[0] if len(parts) > 1 else "unknown"
            parsed = parse_job_from_artifact_dir(artifact_dir)

            artifact_info = {
                "job_name": artifact_dir,  # keep original artifact dir for traceability
                "job": parsed["job"],
                "event": parsed["event"],
                "timestamp": diag_data.get("ts", ""),
                "workflow_run_id": "unknown",  # TODO: wire from manifest builder if available
                "artifact_path": str(relative_path),
                "diagnostics": diag_data,
            }

            artifacts_data.append(artifact_info)

            manifest.append(
                {
                    "job_name": artifact_dir,
                    "job": parsed["job"],
                    "event": parsed["event"],
                    "timestamp": diag_data.get("ts", ""),
                    "artifact_path": str(relative_path),
                    "has_errors": diag_data.get("has_errors", False),
                }
            )

        except Exception as e:
            print(f"Warning: Failed to process {json_file}: {e}", file=sys.stderr)

    return artifacts_data, manifest


def main() -> None:
    parser = argparse.ArgumentParser(description="Analyze diagnostics convergence across workflow runs")
    parser.add_argument("artifacts_dir", help="Directory containing extracted diagnostics artifacts")
    parser.add_argument("--output-dir", default=".", help="Directory to write outputs (default: current directory)")
    args = parser.parse_args()

    if not os.path.isdir(args.artifacts_dir):
        print(f"Error: Artifacts directory not found: {args.artifacts_dir}", file=sys.stderr)
        sys.exit(1)

    print(f"Processing artifacts from: {args.artifacts_dir}", file=sys.stderr)

    artifacts_data, manifest = process_artifacts(args.artifacts_dir)

    if not artifacts_data:
        convergence = {
            "status": "no_data",
            "message": "No diagnostics JSON files found in artifacts directory",
            "total_runs": 0,
        }
    else:
        convergence = analyze_convergence(artifacts_data)

    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    with open(output_dir / "convergence.json", "w") as f:
        json.dump(convergence, f, indent=2)
    print("✓ Written convergence.json", file=sys.stderr)

    markdown_report = generate_markdown_report(convergence)
    with open(output_dir / "convergence.md", "w") as f:
        f.write(markdown_report)
    print("✓ Written convergence.md", file=sys.stderr)

    with open(output_dir / "manifest.json", "w") as f:
        json.dump({"total_artifacts": len(manifest), "artifacts": manifest}, f, indent=2)
    print("✓ Written manifest.json", file=sys.stderr)

    print("\n✅ Convergence analysis complete!", file=sys.stderr)
    print(f"   Analyzed {len(artifacts_data)} diagnostics artifacts", file=sys.stderr)
    print(f"   Trend: {convergence.get('convergence_trend', {}).get('trend', 'N/A')}", file=sys.stderr)


if __name__ == "__main__":
    main()
