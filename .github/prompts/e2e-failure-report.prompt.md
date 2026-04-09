# E2E Failure Report for AgentBaker Vendor E2E Suites

## Description
Generates a comprehensive e2e failure report for a given "Run AgentBaker Vendor E2E Suites" orchestrator pipeline build in Azure DevOps (CloudNativeCompute project, msazure.visualstudio.com).

## Usage
Provide the build ID of the master orchestrator pipeline. If not provided, find the latest completed run of the "Run AgentBaker Vendor E2E Suites" pipeline.

Example: `@workspace /e2e-failure-report buildId=154179906`

---

## Prompt

You are an expert at analyzing Azure DevOps pipeline results for the AgentBaker repo. Your task is to produce a comprehensive e2e failure report for a "Run AgentBaker Vendor E2E Suites" orchestrator build.

### Context
- **ADO Organization**: msazure.visualstudio.com
- **Project**: CloudNativeCompute
- **Pipeline name**: "Run AgentBaker Vendor E2E Suites"
- **Pipeline YAML**: `.pipelines/agentbaker/vendor-tests.yaml`
- **Architecture**: The master pipeline is an orchestrator that triggers child e2e builds via `.pipelines/agentbaker/templates/scripts/e2e.sh` using `az pipelines run --id <SUITE_ID>`. Tests run in the child builds, not the orchestrator itself. The orchestrator monitors child builds and retries failed tests.

### Known Stages and Suite IDs
| Stage | Suite ID |
|-------|----------|
| E2Ev2 AKS RP Master Validation | 138746 |
| AKS E2Ev3 VHD and AgentBakerService PreRelease | 447684 |
| E2Ev2 AKS RP Check-In Test (Default Toggle)-GPU | 140740 |
| E2Ev2 AKS Compatibility Test | 321198 |
| AgentBaker Compatibility Tests | 391827 |
| E2Ev2 AKS Comprehensive Nodepool Snapshot Tests | 402784 |
| E2Ev2 AKS Kata Conformance Tests | 356171 |
| Windows 2022 / 2019 / 2025 / Annual Channel (+ Supportable variants) | Various |
| GPU SKU Coverage Ubuntu / AzureLinux | Various |

### Step-by-Step Analysis Procedure

1. **Get the build**: Use `mcp_ado_pipelines_get_build_status` with the provided `buildId` in the `CloudNativeCompute` project to confirm it's a "Run AgentBaker Vendor E2E Suites" build and get its overall status.

2. **Get the build log index**: Use `mcp_ado_pipelines_get_build_log` to retrieve the log index for the build. This returns a list of all log entries with their IDs and line counts.

3. **Identify stage execution logs**: Each enabled stage has an execution log containing the e2e.sh output. These logs are identifiable by:
   - Medium line count (30-400 lines typically for quick stages, up to several hundred for retried stages)
   - They start with `##[section]Starting: <Stage Name>`
   - They contain `az pipelines run --id <SUITE_ID>` and `Created build id: XXXXXX`

   Read the first few lines of candidate logs using `mcp_ado_pipelines_get_build_log_by_id` to identify which log corresponds to which stage.

4. **For each stage log, extract**:
   - **Child build ID(s)**: Look for `Created build id: XXXXXX`
   - **Build URLs**: Look for `E2E build URL: https://...`
   - **Pass/Fail**: Look for `E2E run XXXXXX succeeded` or `E2E run XXXXXX failed`
   - **Failed test names**: Look for `Failed tests to run-run: <comma-separated test names>`
   - **Retry info**: Look for `N retries remaining` and `ERROR: Exhausted all retries (N/N)`
   - **Skipped stages**: Look for stages where the condition was `eq('False', true)` in the pipeline YAML (log 1)

5. **Classify failures**:
   - **Persistent failures**: Tests that failed across ALL retry attempts
   - **Transient failures**: Tests that failed initially but passed on retry
   - **Infrastructure failures**: Builds that failed with no e2e artifacts (infra issue, not test issue)

6. **Check for common patterns**: If the same test fails across multiple stages, highlight it as a systemic issue.

### Output Format

Produce a report in this exact structure:

```markdown
## E2E Failure Report — Build [BUILD_ID](BUILD_URL)

**Pipeline**: Run AgentBaker Vendor E2E Suites
**Branch**: <branch name>
**Completed**: <completion timestamp>
**Overall Status**: PASSED / FAILED

### Stage Summary

| # | Stage | Child Build(s) | Status |
|---|-------|---------------|--------|
| 1 | Stage Name | [buildId](url) | ✅ Passed / ❌ Failed / ⏭️ Skipped |

### Persistent Failures (root cause)

List each test that failed across all retries, grouped by the stage(s) where it failed.
If the same test fails in multiple stages, call it out as a systemic blocker.

### Transient Failures (resolved on retry)

List tests that failed initially but passed on retry, with the stage name.

### Skipped Stages

List all stages that were disabled/skipped in this run.

### Key Takeaway

One-paragraph summary of the most important finding and recommended action.
```

### Important Notes
- The orchestrator's test results API will return empty — always analyze build logs instead.
- Child builds can be very long-running (hours). The orchestrator polls every 5 minutes.
- The retry mechanism uses `ONLY_RETRY_FAILED_TESTS=True` and `VSTS_TESTS_TO_RUN` to only re-run failed tests.
- Some stages have 3 retries (default), some have 1. Check the `E2E_RETRIES` variable or log output.
- For stages with extra variables (e.g., Compatibility uses `AKS_E2E_SERVICE_VERSION_MATRIX`), note these in the report.
