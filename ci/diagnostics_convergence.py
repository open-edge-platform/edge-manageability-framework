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

Usage:
    python ci/diagnostics_convergence.py <artifacts_dir>

Where artifacts_dir contains extracted diagnostics artifacts with diagnostics_full_*.json files.

Outputs:
- convergence.json: Full convergence analysis with error trends
- convergence.md: Human-readable markdown report
- manifest.json: List of analyzed artifacts with metadata
"""

import argparse
import json
import os
import sys
from collections import defaultdict
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Any


def extract_error_signature(error: Dict[str, Any]) -> str:
    """
    Create a normalized signature for an error to track it across runs.
    Uses namespace, name, and reason to identify unique error patterns.
    """
    namespace = error.get("namespace", "unknown")
    name = error.get("name", "unknown")
    reason = error.get("reason", "unknown")
    return f"{namespace}/{name}:{reason}"


def analyze_convergence(artifacts_data: List[Dict[str, Any]]) -> Dict[str, Any]:
    """
    Analyze error convergence across multiple workflow runs.
    
    Returns:
        Dict containing convergence metrics and trends
    """
    if not artifacts_data:
        return {
            "status": "no_data",
            "message": "No diagnostics data available for analysis",
            "total_runs": 0,
            "error_trends": {}
        }
    
    # Sort by timestamp
    artifacts_data.sort(key=lambda x: x.get("timestamp", ""))
    
    # Track error occurrences across runs
    error_occurrences = defaultdict(list)
    run_summaries = []
    
    for idx, artifact in enumerate(artifacts_data):
        run_num = idx + 1
        diag = artifact.get("diagnostics", {})
        summary = diag.get("summary", {})
        
        # Extract errors from this run
        pods_w_errors = summary.get("pods_w_errors", [])
        deployments_not_ready = summary.get("deployments_not_ready", [])
        argocd_apps_unhealthy = summary.get("argocd_apps_unhealthy", [])
        
        total_errors = len(pods_w_errors) + len(deployments_not_ready) + len(argocd_apps_unhealthy)
        
        run_summaries.append({
            "run": run_num,
            "timestamp": artifact.get("timestamp"),
            "workflow_run_id": artifact.get("workflow_run_id"),
            "job_name": artifact.get("job_name"),
            "total_errors": total_errors,
            "pod_errors": len(pods_w_errors),
            "deployment_errors": len(deployments_not_ready),
            "argocd_errors": len(argocd_apps_unhealthy)
        })
        
        # Track individual error signatures
        for error in pods_w_errors:
            sig = extract_error_signature(error)
            error_occurrences[sig].append({
                "run": run_num,
                "timestamp": artifact.get("timestamp"),
                "details": error
            })
        
        for error in deployments_not_ready:
            sig = extract_error_signature(error)
            error_occurrences[sig].append({
                "run": run_num,
                "timestamp": artifact.get("timestamp"),
                "details": error
            })
        
        for error in argocd_apps_unhealthy:
            sig = extract_error_signature(error)
            error_occurrences[sig].append({
                "run": run_num,
                "timestamp": artifact.get("timestamp"),
                "details": error
            })
    
    # Analyze error patterns
    persistent_errors = []
    transient_errors = []
    
    total_runs = len(artifacts_data)
    for sig, occurrences in error_occurrences.items():
        occurrence_rate = len(occurrences) / total_runs
        
        error_info = {
            "signature": sig,
            "occurrences": len(occurrences),
            "occurrence_rate": occurrence_rate,
            "first_seen": occurrences[0]["timestamp"],
            "last_seen": occurrences[-1]["timestamp"],
            "runs_affected": [occ["run"] for occ in occurrences]
        }
        
        # Persistent = occurs in >50% of runs
        if occurrence_rate > 0.5:
            persistent_errors.append(error_info)
        else:
            transient_errors.append(error_info)
    
    # Calculate convergence trend
    if len(run_summaries) >= 2:
        first_half = run_summaries[:len(run_summaries)//2]
        second_half = run_summaries[len(run_summaries)//2:]
        
        avg_errors_first = sum(r["total_errors"] for r in first_half) / len(first_half)
        avg_errors_second = sum(r["total_errors"] for r in second_half) / len(second_half)
        
        if avg_errors_second < avg_errors_first * 0.8:
            trend = "improving"
        elif avg_errors_second > avg_errors_first * 1.2:
            trend = "degrading"
        else:
            trend = "stable"
    else:
        trend = "insufficient_data"
        avg_errors_first = 0
        avg_errors_second = 0
    
    return {
        "status": "success",
        "total_runs": total_runs,
        "run_summaries": run_summaries,
        "persistent_errors": sorted(persistent_errors, key=lambda x: x["occurrence_rate"], reverse=True),
        "transient_errors": sorted(transient_errors, key=lambda x: x["occurrences"], reverse=True),
        "convergence_trend": {
            "trend": trend,
            "avg_errors_first_half": avg_errors_first,
            "avg_errors_second_half": avg_errors_second
        }
    }


def generate_markdown_report(convergence: Dict[str, Any]) -> str:
    """Generate human-readable markdown report."""
    md = ["# Diagnostics Convergence Analysis\n"]
    
    if convergence["status"] != "success":
        md.append(f"**Status:** {convergence['status']}\n")
        md.append(f"{convergence.get('message', 'Analysis incomplete')}\n")
        return "\n".join(md)
    
    md.append(f"**Analysis Date:** {datetime.now().isoformat()}\n")
    md.append(f"**Total Runs Analyzed:** {convergence['total_runs']}\n\n")
    
    # Convergence trend
    trend = convergence["convergence_trend"]
    md.append("## Convergence Trend\n")
    md.append(f"**Overall Trend:** {trend['trend'].upper()}\n")
    md.append(f"- Average errors (first half): {trend['avg_errors_first_half']:.1f}\n")
    md.append(f"- Average errors (second half): {trend['avg_errors_second_half']:.1f}\n\n")
    
    # Persistent errors
    persistent = convergence["persistent_errors"]
    md.append(f"## Persistent Errors ({len(persistent)} patterns)\n")
    md.append("These errors occur in >50% of runs and require attention:\n\n")
    
    if persistent:
        for err in persistent[:10]:  # Show top 10
            md.append(f"### {err['signature']}\n")
            md.append(f"- **Occurrence Rate:** {err['occurrence_rate']*100:.1f}% ({err['occurrences']}/{convergence['total_runs']} runs)\n")
            md.append(f"- **First Seen:** {err['first_seen']}\n")
            md.append(f"- **Runs Affected:** {', '.join(map(str, err['runs_affected']))}\n\n")
    else:
        md.append("*No persistent errors detected* ✅\n\n")
    
    # Transient errors
    transient = convergence["transient_errors"]
    md.append(f"## Transient Errors ({len(transient)} patterns)\n")
    md.append("These errors occur sporadically (<50% of runs):\n\n")
    
    if transient:
        for err in transient[:5]:  # Show top 5
            md.append(f"- `{err['signature']}`: {err['occurrences']} occurrence(s)\n")
    else:
        md.append("*No transient errors detected* ✅\n")
    
    md.append("\n## Run Summary\n\n")
    md.append("| Run | Timestamp | Job | Total Errors | Pod | Deploy | ArgoCD |\n")
    md.append("|-----|-----------|-----|--------------|-----|--------|--------|\n")
    
    for run in convergence["run_summaries"]:
        md.append(f"| {run['run']} | {run['timestamp'][:19] if run['timestamp'] else 'N/A'} | "
                 f"{run['job_name']} | {run['total_errors']} | {run['pod_errors']} | "
                 f"{run['deployment_errors']} | {run['argocd_errors']} |\n")
    
    return "".join(md)


def process_artifacts(artifacts_dir: str) -> tuple:
    """
    Process all diagnostics artifacts in the directory.
    
    Returns:
        Tuple of (artifacts_data, manifest)
    """
    artifacts_path = Path(artifacts_dir)
    artifacts_data = []
    manifest = []
    
    # Find all diagnostics JSON files
    json_files = list(artifacts_path.glob("**/diagnostics_full_*.json"))
    
    print(f"Found {len(json_files)} diagnostics JSON files", file=sys.stderr)
    
    for json_file in json_files:
        try:
            with open(json_file, 'r') as f:
                diag_data = json.load(f)
            
            # Extract metadata from path or filename
            # Expected structure: job-name/diagnostics_full_timestamp.json
            relative_path = json_file.relative_to(artifacts_path)
            parts = relative_path.parts
            
            job_name = parts[0] if len(parts) > 1 else "unknown"
            
            artifact_info = {
                "job_name": job_name,
                "timestamp": diag_data.get("ts", ""),
                "workflow_run_id": "unknown",  # Will be set by workflow
                "artifact_path": str(relative_path),
                "diagnostics": diag_data
            }
            
            artifacts_data.append(artifact_info)
            
            manifest.append({
                "job_name": job_name,
                "timestamp": diag_data.get("ts", ""),
                "artifact_path": str(relative_path),
                "has_errors": diag_data.get("has_errors", False)
            })
            
        except Exception as e:
            print(f"Warning: Failed to process {json_file}: {e}", file=sys.stderr)
    
    return artifacts_data, manifest


def main():
    parser = argparse.ArgumentParser(
        description="Analyze diagnostics convergence across workflow runs"
    )
    parser.add_argument(
        "artifacts_dir",
        help="Directory containing extracted diagnostics artifacts"
    )
    parser.add_argument(
        "--output-dir",
        default=".",
        help="Directory to write convergence outputs (default: current directory)"
    )
    
    args = parser.parse_args()
    
    if not os.path.isdir(args.artifacts_dir):
        print(f"Error: Artifacts directory not found: {args.artifacts_dir}", file=sys.stderr)
        sys.exit(1)
    
    print(f"Processing artifacts from: {args.artifacts_dir}", file=sys.stderr)
    
    # Process artifacts
    artifacts_data, manifest = process_artifacts(args.artifacts_dir)
    
    if not artifacts_data:
        print("Warning: No diagnostics data found", file=sys.stderr)
        convergence = {
            "status": "no_data",
            "message": "No diagnostics JSON files found in artifacts directory",
            "total_runs": 0,
            "error_trends": {}
        }
    else:
        # Analyze convergence
        convergence = analyze_convergence(artifacts_data)
    
    # Write outputs
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    
    # convergence.json
    with open(output_dir / "convergence.json", "w") as f:
        json.dump(convergence, f, indent=2)
    print(f"✓ Written convergence.json", file=sys.stderr)
    
    # convergence.md
    markdown_report = generate_markdown_report(convergence)
    with open(output_dir / "convergence.md", "w") as f:
        f.write(markdown_report)
    print(f"✓ Written convergence.md", file=sys.stderr)
    
    # manifest.json
    with open(output_dir / "manifest.json", "w") as f:
        json.dump({
            "total_artifacts": len(manifest),
            "artifacts": manifest
        }, f, indent=2)
    print(f"✓ Written manifest.json", file=sys.stderr)
    
    print(f"\n✅ Convergence analysis complete!", file=sys.stderr)
    print(f"   Analyzed {len(artifacts_data)} diagnostics artifacts", file=sys.stderr)
    print(f"   Trend: {convergence.get('convergence_trend', {}).get('trend', 'N/A')}", file=sys.stderr)


if __name__ == "__main__":
    main()
