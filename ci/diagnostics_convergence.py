#!/usr/bin/env python3
# SPDX-FileCopyrightText: (C) 2026 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

"""
======================================================================================
Diagnostics Convergence Analyzer
======================================================================================

Analyzes diagnostics across multiple workflow runs to identify recurring failure
signatures and patterns. Used for automated convergence after CI/CD failures.

Purpose:
--------
- Scans diagnostics artifacts from multiple workflow runs
- Generates stable failure signatures (normalized to ignore transient details)
- Aggregates failures by frequency and co-occurrence
- Produces machine-readable JSON and human-readable Markdown reports

Signature Generation Rules:
---------------------------
1. Pod Errors:
   - Uses reason (if present) or status
   - Normalizes message/last_event by stripping timestamps, IPs, UUIDs, image digests, pod RS suffixes
   
2. Deployments Not Ready:
   - Signature: deploy_not_ready::<namespace>::<name>::<reason>
   - Normalizes reason field
   
3. ArgoCD Unhealthy:
   - Signature: argocd::<namespace>::<name>::<health>::<sync>::<operation_phase>
   - Optional normalized message hash for additional context
   
4. PVC Not Bound:
   - Signature: pvc::<namespace>::<pvc>::<status>

Usage:
------
python ci/diagnostics_convergence.py \\
    --manifest manifest.json \\
    --artifacts-dir ./artifacts \\
    --output convergence.json \\
    --output-md convergence.md
"""

import argparse
import hashlib
import json
import os
import re
import sys
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional, Set, Tuple


# Normalization regex patterns (module-level constants for maintainability)
# ISO8601/RFC3339 timestamp: YYYY-MM-DD[T ]HH:MM:SS[.microseconds][Z|+HH:MM]
TIMESTAMP_ISO_PATTERN = r'\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?'
# Unix timestamp (10 digits, e.g., 1234567890)
TIMESTAMP_UNIX_PATTERN = r'\b\d{10}\b'
# IPv4 address (simple pattern, e.g., 192.168.1.1)
IPV4_PATTERN = r'\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b'
# IPv6 address (simplified pattern matching colon-separated hex groups)
# Matches full and compressed forms like 2001:db8::1 or 2001:0db8:0000:0000:0000:ff00:0042:8329
IPV6_PATTERN = r'\b(?:[0-9a-fA-F]{1,4}:){2,7}[0-9a-fA-F]{0,4}\b'
# UUID (8-4-4-4-12 format, e.g., 550e8400-e29b-41d4-a716-446655440000)
UUID_PATTERN = r'\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b'
# Container image digest (sha256:...)
IMAGE_DIGEST_PATTERN = r'sha256:[0-9a-fA-F]{64}'
# Pod ReplicaSet suffix (Kubernetes template hash is exactly 10 chars, pod suffix is 5 chars)
# Example: pod-name-abc1234567-xyz12 -> pod-name-RS
POD_RS_SUFFIX_PATTERN = r'-[a-z0-9]{10}-[a-z0-9]{5}\b'
# Hexadecimal IDs (8+ characters, often used for container IDs, etc.)
HEXID_PATTERN = r'\b[0-9a-fA-F]{8,}\b'


def normalize_text(text: str) -> str:
    """
    Normalize text by removing transient details:
    - Timestamps (ISO8601, RFC3339, Unix timestamps)
    - IP addresses (IPv4, IPv6)
    - UUIDs
    - Container image digests (sha256:...)
    - Pod ReplicaSet suffixes (-[a-z0-9]{10}-[a-z0-9]{5})
    - Hexadecimal IDs
    
    Args:
        text: Original text to normalize
        
    Returns:
        Normalized text with transient details removed
    """
    if not text:
        return ""
    
    # Remove timestamps (ISO8601/RFC3339)
    text = re.sub(TIMESTAMP_ISO_PATTERN, 'TIMESTAMP', text)
    
    # Remove Unix timestamps (10 digits)
    text = re.sub(TIMESTAMP_UNIX_PATTERN, 'TIMESTAMP', text)
    
    # Remove IPv4 addresses
    text = re.sub(IPV4_PATTERN, 'IP', text)
    
    # Remove IPv6 addresses
    text = re.sub(IPV6_PATTERN, 'IP', text)
    
    # Remove UUIDs
    text = re.sub(UUID_PATTERN, 'UUID', text)
    
    # Remove image digests
    text = re.sub(IMAGE_DIGEST_PATTERN, 'DIGEST', text)
    
    # Remove pod ReplicaSet suffixes (e.g., pod-name-abc1234567-xyz12 -> pod-name-RS)
    text = re.sub(POD_RS_SUFFIX_PATTERN, '-RS', text)
    
    # Remove hexadecimal IDs (8+ chars)
    text = re.sub(HEXID_PATTERN, 'HEXID', text)
    
    return text


def compute_hash(text: str, length: int = 8) -> str:
    """
    Compute a short hash of normalized text for signature deduplication.
    
    Args:
        text: Text to hash
        length: Length of hash to return (default: 8 chars)
        
    Returns:
        Short hash string
    """
    if not text:
        return "no_msg"
    normalized = normalize_text(text)
    return hashlib.sha256(normalized.encode()).hexdigest()[:length]


def generate_pod_error_signature(pod_error: Dict[str, Any]) -> str:
    """
    Generate a stable signature for a pod error.
    
    Uses reason if available, otherwise status. Adds normalized message hash
    for additional disambiguation.
    
    Args:
        pod_error: Pod error dict from diagnostics JSON
        
    Returns:
        Signature string
    """
    namespace = pod_error.get("namespace", "unknown")
    pod = pod_error.get("pod", "unknown")
    reason = pod_error.get("reason", "")
    status = pod_error.get("status", "Unknown")
    message = pod_error.get("message", "")
    last_event = pod_error.get("last_event", "")
    
    # Use reason if available, otherwise status
    primary_indicator = reason if reason else status
    
    # Normalize pod name to remove RS suffix (exact Kubernetes pattern)
    pod_normalized = re.sub(r'-[a-z0-9]{10}-[a-z0-9]{5}$', '', pod)
    
    # Compute hash of message/event for additional context
    text_for_hash = message if message else last_event
    msg_hash = compute_hash(text_for_hash)
    
    return f"pod_error::{namespace}::{pod_normalized}::{primary_indicator}::{msg_hash}"


def generate_deployment_signature(deployment: Dict[str, Any]) -> str:
    """
    Generate a stable signature for a deployment not ready issue.
    
    Args:
        deployment: Deployment issue dict from diagnostics JSON
        
    Returns:
        Signature string
    """
    namespace = deployment.get("namespace", "unknown")
    name = deployment.get("name", "unknown")
    reason = deployment.get("reason", "Unknown")
    
    # Normalize reason (remove transient details)
    reason_normalized = normalize_text(reason)
    
    return f"deploy_not_ready::{namespace}::{name}::{reason_normalized}"


def generate_argocd_signature(argocd_app: Dict[str, Any]) -> str:
    """
    Generate a stable signature for an ArgoCD unhealthy application.
    
    Args:
        argocd_app: ArgoCD app dict from diagnostics JSON
        
    Returns:
        Signature string
    """
    namespace = argocd_app.get("namespace", "unknown")
    name = argocd_app.get("name", "unknown")
    health = argocd_app.get("health", "Unknown")
    sync = argocd_app.get("sync", "Unknown")
    operation_phase = argocd_app.get("operation_phase", "N/A")
    message = argocd_app.get("message", "")
    
    # Compute normalized message hash for additional context
    msg_hash = compute_hash(message)
    
    return f"argocd::{namespace}::{name}::{health}::{sync}::{operation_phase}::{msg_hash}"


def generate_pvc_signature(pvc: Dict[str, Any]) -> str:
    """
    Generate a stable signature for a PVC not bound issue.
    
    Args:
        pvc: PVC issue dict from diagnostics JSON
        
    Returns:
        Signature string
    """
    namespace = pvc.get("namespace", "unknown")
    pvc_name = pvc.get("pvc", "unknown")
    status = pvc.get("status", "Unknown")
    
    return f"pvc::{namespace}::{pvc_name}::{status}"


def parse_diagnostics_file(filepath: Path) -> Optional[Dict[str, Any]]:
    """
    Parse a diagnostics_full_*.json file.
    
    Args:
        filepath: Path to diagnostics JSON file
        
    Returns:
        Parsed JSON dict or None if parsing fails
    """
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            return json.load(f)
    except (json.JSONDecodeError, IOError) as e:
        print(f"Warning: Failed to parse {filepath}: {e}", file=sys.stderr)
        return None


def extract_signatures_from_diagnostics(diag_data: Dict[str, Any], run_metadata: Dict[str, Any], job_name: str) -> Dict[str, List[Dict[str, Any]]]:
    """
    Extract failure signatures from a diagnostics JSON file.
    
    Args:
        diag_data: Parsed diagnostics JSON
        run_metadata: Metadata about the workflow run
        job_name: Name of the job (e.g., deploy-kind)
        
    Returns:
        Dict mapping signature category to list of signature dicts
    """
    signatures = {
        "pod_errors": [],
        "deployments_not_ready": [],
        "argocd_apps_unhealthy": [],
        "pvc_not_bound": []
    }
    
    summary = diag_data.get("summary", {})
    
    # Extract pod errors
    for pod_error in summary.get("pods_w_errors", []):
        sig = generate_pod_error_signature(pod_error)
        signatures["pod_errors"].append({
            "signature": sig,
            "run_id": run_metadata.get("run_id"),
            "run_url": run_metadata.get("html_url"),
            "job": job_name,
            "details": {
                "namespace": pod_error.get("namespace"),
                "pod": pod_error.get("pod"),
                "status": pod_error.get("status"),
                "reason": pod_error.get("reason"),
                "message": pod_error.get("message", "")[:200],  # Truncate for readability
            }
        })
    
    # Extract deployment issues
    for deployment in summary.get("deployments_not_ready", []):
        sig = generate_deployment_signature(deployment)
        signatures["deployments_not_ready"].append({
            "signature": sig,
            "run_id": run_metadata.get("run_id"),
            "run_url": run_metadata.get("html_url"),
            "job": job_name,
            "details": {
                "namespace": deployment.get("namespace"),
                "name": deployment.get("name"),
                "reason": deployment.get("reason"),
                "expected": deployment.get("expected"),
                "ready": deployment.get("ready"),
            }
        })
    
    # Extract ArgoCD issues
    for argocd_app in summary.get("argocd_apps_unhealthy", []):
        sig = generate_argocd_signature(argocd_app)
        signatures["argocd_apps_unhealthy"].append({
            "signature": sig,
            "run_id": run_metadata.get("run_id"),
            "run_url": run_metadata.get("html_url"),
            "job": job_name,
            "details": {
                "namespace": argocd_app.get("namespace"),
                "name": argocd_app.get("name"),
                "health": argocd_app.get("health"),
                "sync": argocd_app.get("sync"),
                "operation_phase": argocd_app.get("operation_phase"),
                "message": argocd_app.get("message", "")[:200],  # Truncate for readability
            }
        })
    
    # Extract PVC issues
    for pvc in summary.get("pvc_not_bound", []):
        sig = generate_pvc_signature(pvc)
        signatures["pvc_not_bound"].append({
            "signature": sig,
            "run_id": run_metadata.get("run_id"),
            "run_url": run_metadata.get("html_url"),
            "job": job_name,
            "details": {
                "namespace": pvc.get("namespace"),
                "pvc": pvc.get("pvc"),
                "status": pvc.get("status"),
            }
        })
    
    return signatures


def aggregate_signatures(all_signatures: List[Dict[str, List[Dict[str, Any]]]]) -> Dict[str, Dict[str, Any]]:
    """
    Aggregate signatures across all runs.
    
    Computes:
    - Frequency count per signature per job
    - Example runs for each signature
    - Co-occurrence analysis (top 3 signatures that occur in same run+job)
    
    Args:
        all_signatures: List of signature dicts from all runs
        
    Returns:
        Aggregated results dict
    """
    # Track signature occurrences by category and job
    signature_data = defaultdict(lambda: {
        "count": 0,
        "jobs": defaultdict(int),
        "example_runs": [],
        "run_ids": set(),
        "details_samples": []
    })
    
    # Track co-occurrence: (run_id, job) -> set of signatures
    run_job_signatures = defaultdict(set)
    
    for sig_dict in all_signatures:
        for category, signatures in sig_dict.items():
            for sig_entry in signatures:
                sig = sig_entry["signature"]
                run_id = sig_entry["run_id"]
                job = sig_entry["job"]
                run_url = sig_entry["run_url"]
                details = sig_entry["details"]
                
                # Create key for this signature in this category
                key = f"{category}::{sig}"
                
                # Update counts
                signature_data[key]["count"] += 1
                signature_data[key]["jobs"][job] += 1
                signature_data[key]["category"] = category
                signature_data[key]["signature"] = sig
                
                # Add example run (up to 5 examples)
                if run_id not in signature_data[key]["run_ids"]:
                    signature_data[key]["run_ids"].add(run_id)
                    if len(signature_data[key]["example_runs"]) < 5:
                        signature_data[key]["example_runs"].append({
                            "run_id": run_id,
                            "run_url": run_url,
                            "job": job
                        })
                
                # Add details sample (up to 3)
                if len(signature_data[key]["details_samples"]) < 3:
                    signature_data[key]["details_samples"].append(details)
                
                # Track for co-occurrence
                run_job_key = (run_id, job)
                run_job_signatures[run_job_key].add(key)
    
    # Compute co-occurrence for each signature
    for key, data in signature_data.items():
        co_occur_counts = defaultdict(int)
        
        # Find all run+job combinations where this signature occurred
        for run_job_key, sigs in run_job_signatures.items():
            if key in sigs:
                # Count co-occurrences with other signatures
                for other_sig in sigs:
                    if other_sig != key:
                        co_occur_counts[other_sig] += 1
        
        # Get top 3 co-occurring signatures
        top_co_occur = sorted(co_occur_counts.items(), key=lambda x: x[1], reverse=True)[:3]
        data["co_occurrence"] = [
            {"signature": sig, "count": count}
            for sig, count in top_co_occur
        ]
        
        # Convert run_ids set to count for JSON serialization
        data["unique_runs"] = len(data["run_ids"])
        del data["run_ids"]
    
    # Convert defaultdicts to regular dicts for JSON serialization
    result = {}
    for key, data in signature_data.items():
        data["jobs"] = dict(data["jobs"])
        result[key] = data
    
    return result


def generate_json_report(aggregated: Dict[str, Any], metadata: Dict[str, Any], output_path: Path):
    """
    Generate machine-readable JSON convergence report.
    
    Args:
        aggregated: Aggregated signature data
        metadata: Report metadata (filters, run count, etc.)
        output_path: Path to write JSON file
    """
    report = {
        "generated_at": datetime.now(timezone.utc).isoformat().replace('+00:00', 'Z'),
        "metadata": metadata,
        "signatures": aggregated
    }
    
    with open(output_path, 'w', encoding='utf-8') as f:
        json.dump(report, f, indent=2)
    
    print(f"✓ Generated JSON report: {output_path}")


def generate_markdown_report(aggregated: Dict[str, Any], metadata: Dict[str, Any], output_path: Path):
    """
    Generate human-readable Markdown convergence report.
    
    Args:
        aggregated: Aggregated signature data
        metadata: Report metadata (filters, run count, etc.)
        output_path: Path to write Markdown file
    """
    lines = []
    
    # Header
    lines.append("# Diagnostics Convergence Report")
    lines.append("")
    lines.append(f"**Generated:** {datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M:%S')} UTC")
    lines.append("")
    
    # Metadata
    lines.append("## Analysis Scope")
    lines.append("")
    lines.append(f"- **Runs analyzed:** {metadata.get('runs_scanned', 0)}")
    lines.append(f"- **Jobs included:** {', '.join(metadata.get('jobs_filter', []))}")
    
    branch_filter = metadata.get('branch_filter')
    event_filter = metadata.get('event_filter')
    if branch_filter:
        lines.append(f"- **Branch filter:** {branch_filter}")
    if event_filter:
        lines.append(f"- **Event filter:** {event_filter}")
    
    lines.append("")
    
    # Group by category
    categories = {
        "pod_errors": "Pod Errors",
        "deployments_not_ready": "Deployments Not Ready",
        "argocd_apps_unhealthy": "ArgoCD Applications Unhealthy",
        "pvc_not_bound": "PVCs Not Bound"
    }
    
    for category_key, category_title in categories.items():
        # Filter signatures for this category
        category_sigs = {
            k: v for k, v in aggregated.items()
            if v.get("category") == category_key
        }
        
        if not category_sigs:
            continue
        
        lines.append(f"## {category_title}")
        lines.append("")
        
        # Sort by frequency (descending)
        sorted_sigs = sorted(category_sigs.items(), key=lambda x: x[1]["count"], reverse=True)
        
        # Create table
        lines.append("| Rank | Signature | Frequency | Jobs | Example Runs |")
        lines.append("|------|-----------|-----------|------|--------------|")
        
        for rank, (key, data) in enumerate(sorted_sigs[:20], start=1):  # Top 20
            sig = data["signature"]
            count = data["count"]
            jobs_str = ", ".join([f"{j}({c})" for j, c in data["jobs"].items()])
            
            # Format example runs as links
            examples = data["example_runs"][:3]
            example_links = []
            for ex in examples:
                run_id = ex["run_id"]
                run_url = ex["run_url"]
                job = ex["job"]
                example_links.append(f"[{run_id}]({run_url}) ({job})")
            examples_str = "<br>".join(example_links)
            
            lines.append(f"| {rank} | `{sig}` | {count} | {jobs_str} | {examples_str} |")
        
        lines.append("")
        
        # Co-occurrence section
        if sorted_sigs:
            lines.append(f"### Top {category_title} - Co-occurrence Patterns")
            lines.append("")
            
            # Show co-occurrence for top 5 most frequent signatures
            for rank, (key, data) in enumerate(sorted_sigs[:5], start=1):
                sig = data["signature"]
                co_occur = data.get("co_occurrence", [])
                
                if co_occur:
                    lines.append(f"**{rank}. {sig}**")
                    lines.append("")
                    lines.append("Often occurs with:")
                    for co in co_occur:
                        co_sig = co["signature"].split("::", 1)[1]  # Remove category prefix
                        co_count = co["count"]
                        lines.append(f"- `{co_sig}` ({co_count} times)")
                    lines.append("")
        
        lines.append("---")
        lines.append("")
    
    # Summary statistics
    lines.append("## Summary Statistics")
    lines.append("")
    lines.append(f"- **Total unique signatures:** {len(aggregated)}")
    
    # Count by category
    for category_key, category_title in categories.items():
        count = sum(1 for v in aggregated.values() if v.get("category") == category_key)
        if count > 0:
            lines.append(f"- **{category_title}:** {count}")
    
    lines.append("")
    
    # Write to file
    with open(output_path, 'w', encoding='utf-8') as f:
        f.write("\n".join(lines))
    
    print(f"✓ Generated Markdown report: {output_path}")


def main():
    """Main entry point for diagnostics convergence analysis."""
    parser = argparse.ArgumentParser(
        description="Analyze diagnostics across multiple workflow runs to identify recurring failure signatures"
    )
    parser.add_argument(
        "--manifest",
        required=True,
        type=Path,
        help="Path to manifest JSON file containing run and artifact metadata"
    )
    parser.add_argument(
        "--artifacts-dir",
        required=True,
        type=Path,
        help="Directory containing extracted artifact folders"
    )
    parser.add_argument(
        "--output",
        required=True,
        type=Path,
        help="Path to write convergence JSON output"
    )
    parser.add_argument(
        "--output-md",
        required=True,
        type=Path,
        help="Path to write convergence Markdown output"
    )
    
    args = parser.parse_args()
    
    # Load manifest
    print(f"Loading manifest from {args.manifest}...")
    try:
        with open(args.manifest, 'r', encoding='utf-8') as f:
            manifest = json.load(f)
    except (json.JSONDecodeError, IOError) as e:
        print(f"Error: Failed to load manifest: {e}", file=sys.stderr)
        sys.exit(1)
    
    runs = manifest.get("runs", [])
    metadata = manifest.get("metadata", {})
    
    print(f"✓ Loaded manifest with {len(runs)} runs")
    
    # Process each run and extract signatures
    all_signatures = []
    processed_count = 0
    
    for run_entry in runs:
        run_metadata = run_entry.get("run", {})
        artifacts = run_entry.get("artifacts", [])
        
        for artifact in artifacts:
            artifact_name = artifact.get("name", "")
            artifact_id = artifact.get("id", "")
            
            # Determine job name from artifact name
            # Format: diagnostics-<job>-<event>-<number>-<attempt>
            # Job names can contain hyphens (e.g., deploy-kind, deploy-on-prem)
            # We need to match against known jobs from metadata to extract correctly
            jobs_filter = metadata.get("jobs_filter", [])
            job_name = None
            
            if artifact_name.startswith('diagnostics-'):
                # Try to match against known jobs
                for known_job in jobs_filter:
                    if artifact_name.startswith(f'diagnostics-{known_job}-'):
                        job_name = known_job
                        break
                
                # Fallback: if no match found, try common event types to extract job
                if not job_name:
                    # Try matching with common event types
                    event_pattern = r'^diagnostics-(.*?)-(push|pull_request|merge_group|workflow_dispatch|schedule)-'
                    match = re.match(event_pattern, artifact_name)
                    if match:
                        job_name = match.group(1)
            
            if not job_name:
                print(f"Warning: Could not extract job name from artifact '{artifact_name}'", file=sys.stderr)
                continue
            
            # Find diagnostics_full_*.json in artifact directory
            artifact_dir = args.artifacts_dir / f"artifact-{artifact_id}"
            if not artifact_dir.exists():
                print(f"Warning: Artifact directory not found: {artifact_dir}", file=sys.stderr)
                continue
            
            # Search for diagnostics_full_*.json files
            diag_files = list(artifact_dir.glob("diagnostics_full_*.json"))
            
            if not diag_files:
                print(f"Warning: No diagnostics_full_*.json found in {artifact_dir}", file=sys.stderr)
                continue
            
            # Process the first diagnostics file found
            diag_file = diag_files[0]
            diag_data = parse_diagnostics_file(diag_file)
            
            if diag_data:
                signatures = extract_signatures_from_diagnostics(diag_data, run_metadata, job_name)
                all_signatures.append(signatures)
                processed_count += 1
                print(f"✓ Processed {diag_file.name} from run {run_metadata.get('run_id')} (job: {job_name})")
    
    print(f"\n✓ Processed {processed_count} diagnostics files from {len(runs)} runs")
    
    if not all_signatures:
        print("Warning: No signatures extracted. Nothing to aggregate.", file=sys.stderr)
        # Create empty reports
        empty_aggregated = {}
        generate_json_report(empty_aggregated, metadata, args.output)
        generate_markdown_report(empty_aggregated, metadata, args.output_md)
        return
    
    # Aggregate signatures
    print("\nAggregating signatures...")
    aggregated = aggregate_signatures(all_signatures)
    print(f"✓ Identified {len(aggregated)} unique failure signatures")
    
    # Generate reports
    print("\nGenerating reports...")
    metadata["runs_scanned"] = len(runs)
    metadata["diagnostics_processed"] = processed_count
    
    generate_json_report(aggregated, metadata, args.output)
    generate_markdown_report(aggregated, metadata, args.output_md)
    
    print("\n✓ Convergence analysis complete!")


if __name__ == "__main__":
    main()
