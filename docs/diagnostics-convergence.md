# Diagnostics Convergence Workflow

## Overview

The Diagnostics Convergence workflow is an automated analysis tool that identifies recurring failure patterns across multiple workflow runs. It helps developers and operators quickly understand the most common failure modes by aggregating and analyzing diagnostics data from CI/CD pipeline failures.

## Purpose

When the Virtual Integration workflow fails, it generates detailed diagnostics artifacts containing information about:
- Pod errors (CrashLoopBackOff, ImagePullBackOff, etc.)
- Deployment readiness issues
- ArgoCD application health problems
- PVC binding failures

The Convergence workflow collects these diagnostics from multiple runs, normalizes them to create stable failure signatures, and generates reports showing:
- Most frequent failure patterns
- Which jobs experience which failures
- Co-occurrence patterns (failures that often happen together)
- Example runs for each failure type

## How It Works

### Automatic Triggering

The workflow automatically runs whenever the Virtual Integration workflow completes with a non-success status (failed, cancelled, timed out, etc.). It analyzes the last 20 workflow runs by default.

### Manual Triggering

You can also trigger the workflow manually to analyze specific scenarios:

1. Go to the Actions tab in GitHub
2. Select "Diagnostics Convergence" workflow
3. Click "Run workflow"
4. Configure optional filters:
   - **Branch**: Analyze only runs from a specific branch (leave empty for all branches)
   - **Event**: Filter by event type like `push`, `pull_request`, etc. (leave empty for all events)
   - **Runs**: Number of recent runs to analyze (default: 20)
   - **Jobs**: Comma-separated list of jobs to include (default: `deploy-kind,deploy-on-prem,deploy-oxm-profile`)

### Analysis Process

1. **Query GitHub API**: The workflow queries the GitHub Actions API to retrieve the last N workflow runs of the Virtual Integration workflow
2. **Download Artifacts**: Downloads diagnostics artifacts from each run that match the selected jobs
3. **Extract Signatures**: Parses `diagnostics_full_*.json` files and generates normalized failure signatures
4. **Aggregate Results**: Counts frequency, identifies co-occurrence patterns, and collects example runs
5. **Generate Reports**: Creates both machine-readable JSON and human-readable Markdown reports

### Failure Signature Normalization

To identify recurring patterns, the tool normalizes failure data by removing transient details:

- **Timestamps**: All timestamp formats are replaced with `TIMESTAMP`
- **IP Addresses**: Both IPv4 and IPv6 addresses are replaced with `IP`
- **UUIDs**: Universally unique identifiers are replaced with `UUID`
- **Image Digests**: SHA256 digests are replaced with `DIGEST`
- **Pod ReplicaSet Suffixes**: Dynamic pod suffixes (e.g., `-5f8b9c7d6-xkz9p`) are replaced with `-RS`
- **Hexadecimal IDs**: Long hex strings are replaced with `HEXID`

This normalization ensures that the same underlying issue is recognized across different runs, even when transient details differ.

## Output Reports

### convergence.json (Machine-Readable)

A structured JSON file containing:
- `metadata`: Analysis configuration (filters, run count, timestamps)
- `signatures`: Detailed data for each unique failure signature including:
  - Frequency count
  - Per-job breakdown
  - Example runs with URLs
  - Co-occurrence patterns
  - Sample details from actual failures

This file can be used for:
- Automated alerting
- Trend analysis over time
- Integration with monitoring systems
- Custom reporting tools

### convergence.md (Human-Readable)

A Markdown report with:
- **Analysis Scope**: Configuration and filters applied
- **Per-Category Tables**: Ranked list of failures by frequency for:
  - Pod Errors
  - Deployments Not Ready
  - ArgoCD Applications Unhealthy
  - PVCs Not Bound
- **Co-occurrence Patterns**: Which failures tend to happen together
- **Summary Statistics**: Overall counts by category

The Markdown report is also displayed in the workflow run summary for quick viewing.

## Accessing Reports

After the workflow completes:

1. **View Summary**: The Markdown report is displayed in the workflow run summary (click on the workflow run in the Actions tab)
2. **Download Artifacts**: Full reports are uploaded as workflow artifacts:
   - Navigate to the workflow run
   - Scroll to the "Artifacts" section at the bottom
   - Download `convergence-reports-<run_number>-<run_attempt>`
   - Extract the zip to access `convergence.json`, `convergence.md`, and `manifest.json`

## Example Use Cases

### Identify Flaky Tests
Run the convergence analysis on the last 50 runs to see which failures are most common:
```
Branch: (empty - all branches)
Event: pull_request
Runs: 50
Jobs: deploy-kind,deploy-on-prem,deploy-oxm-profile
```

### Debug Branch-Specific Issues
Analyze failures on a specific branch:
```
Branch: feature/my-branch
Event: (empty - all events)
Runs: 20
Jobs: deploy-kind,deploy-on-prem,deploy-oxm-profile
```

### Focus on Specific Job
Analyze only one job type:
```
Branch: (empty)
Event: (empty)
Runs: 30
Jobs: deploy-kind
```

### Historical Analysis
Look at a larger time window:
```
Branch: main
Event: push
Runs: 100
Jobs: deploy-kind,deploy-on-prem,deploy-oxm-profile
```

## Understanding the Reports

### Signature Format

Each failure type has a specific signature format:

- **Pod Errors**: `pod_error::<namespace>::<pod_name>::<reason_or_status>::<message_hash>`
  - Example: `pod_error::kube-system::coredns::CrashLoopBackOff::a1b2c3d4`

- **Deployments Not Ready**: `deploy_not_ready::<namespace>::<deployment_name>::<reason>`
  - Example: `deploy_not_ready::default::myapp::ProgressDeadlineExceeded`

- **ArgoCD Unhealthy**: `argocd::<namespace>::<app_name>::<health>::<sync>::<operation_phase>::<message_hash>`
  - Example: `argocd::argocd::myapp::Degraded::OutOfSync::Failed::x7y8z9w0`

- **PVC Not Bound**: `pvc::<namespace>::<pvc_name>::<status>`
  - Example: `pvc::default::data-postgres-0::Pending`

### Frequency Interpretation

- **High Frequency (>80%)**: Failure occurs in most runs, likely a systematic issue
- **Medium Frequency (30-80%)**: Intermittent but recurring, possible race condition or timing issue
- **Low Frequency (<30%)**: Rare occurrence, might be environmental or truly flaky

### Co-occurrence Patterns

Co-occurrence data helps identify root causes:
- If "pod error A" and "deployment not ready B" often occur together, B might be causing A
- Analyze the most frequent co-occurrences first as they likely represent primary failure modes

## Troubleshooting

### No Artifacts Found

If the workflow reports "No workflow runs with diagnostics artifacts found":
- Ensure the Virtual Integration workflow has run recently
- Check that the `collect_diagnostics` action is configured in the failing jobs
- Verify the artifact naming matches the expected pattern: `diagnostics-<job>-*`

### Workflow Fails to Download Artifacts

- Check that the workflow has `actions: read` permission
- Verify the GitHub token has access to the repository
- Some artifacts may have expired (default retention is 15 days)

### Empty Convergence Report

If the report shows no signatures:
- Check that `diagnostics_full_*.json` files exist in the downloaded artifacts
- Verify the JSON structure matches the expected format
- Look for warnings in the workflow logs about parsing failures

## Technical Details

### Permissions Required

The workflow requires minimal permissions:
- `actions: read` - To query workflow runs and download artifacts
- `contents: read` - To checkout the repository code

### Dependencies

- Python 3.11 or higher
- GitHub Actions API access
- Access to workflow run artifacts

### Performance

- Analysis of 20 runs typically takes 2-5 minutes
- Analysis of 100 runs may take 10-15 minutes
- Large artifacts (>100MB each) will increase processing time

## Contributing

To modify or extend the convergence analysis:

1. **Workflow File**: `.github/workflows/diagnostics-convergence.yml`
   - Modify triggers, inputs, or artifact download logic

2. **Analysis Script**: `ci/diagnostics_convergence.py`
   - Add new signature types
   - Enhance normalization rules
   - Customize report formats

3. **Testing**: Create sample diagnostics JSON files and test locally:
   ```bash
   python ci/diagnostics_convergence.py \
     --manifest test_manifest.json \
     --artifacts-dir test_artifacts \
     --output convergence.json \
     --output-md convergence.md
   ```

## Additional Resources

- [Virtual Integration Workflow](.github/workflows/virtual-integration.yml)
- [Collect Diagnostics Action](.github/actions/collect_diagnostics/action.yaml)
- [Diagnostics Script](ci/orch_k8s_diagnostics.py)
- [GitHub Actions API Documentation](https://docs.github.com/en/rest/actions)
