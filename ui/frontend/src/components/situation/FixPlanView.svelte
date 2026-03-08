<script>
  import { selectedIncidentId } from '../../lib/stores/graphStore';
  import { fetchFixPlan, runCommand, runScan, recordFixAttempt, postCheck } from '../../lib/api/wailsBridge';
  import type { PostCheckResult } from '../../lib/types/diagnostics';

  export let onBack = () => {};

  let plan = null;
  let loading = false;
  let error = null;
  let cmdResults = {}; // keyed by command string
  let runningCmds = {}; // keyed by command string
  let suggestedCmds = []; // dynamically suggested commands from analysis
  let postCheckResults: PostCheckResult[] = [];
  let postChecking = false;

  $: if ($selectedIncidentId) {
    loadPlan($selectedIncidentId);
  }

  async function loadPlan(incidentId) {
    loading = true;
    error = null;
    cmdResults = {};
    runningCmds = {};
    suggestedCmds = [];
    try {
      plan = await fetchFixPlan(incidentId);
      if (!plan) error = 'No fix plan generated';
    } catch (e) {
      error = e?.message || String(e);
    } finally {
      loading = false;
    }
  }

  async function rescanAndDiagnose() {
    loading = true;
    error = null;
    cmdResults = {};
    runningCmds = {};
    suggestedCmds = [];
    try {
      await runScan();
      plan = await fetchFixPlan($selectedIncidentId);
      if (!plan) error = 'No fix plan generated — issue may be resolved';
    } catch (e) {
      error = e?.message || String(e);
    } finally {
      loading = false;
    }
  }

  function copyCmd(cmd) {
    navigator.clipboard.writeText(cmd);
  }

  // Collect all error codes from a step for context
  function stepErrorCodes(step) {
    return step.errorCodes || [];
  }

  async function runCmd(cmd, errorCodes, step = null, isDestructive = false) {
    runningCmds[cmd] = true;
    runningCmds = runningCmds;
    try {
      const result = await runCommand(cmd, errorCodes || []);
      cmdResults[cmd] = result;
      cmdResults = cmdResults;

      // Auto-record destructive commands for learning
      if (isDestructive && step) {
        recordFixAttempt({
          errorCode: step.errorCodes?.[0] || '',
          namespace: step.namespace || '',
          resourceKind: step.resourceKind || '',
          resourceName: step.resourceName || '',
          command: cmd,
          treePath: step.treePath || '',
          optionLabel: '',
          success: result.success,
          output: result.output || result.error || '',
        });
      }

      // Add suggested next commands from analysis
      if (result.suggestion?.nextCmds?.length > 0) {
        for (const next of result.suggestion.nextCmds) {
          if (!suggestedCmds.find(s => s.command === next.command)) {
            suggestedCmds = [...suggestedCmds, next];
          }
        }
      }
    } finally {
      delete runningCmds[cmd];
      runningCmds = runningCmds;
    }
  }

  async function runPostCheck() {
    if (!$selectedIncidentId) return;
    postChecking = true;
    try {
      postCheckResults = await postCheck($selectedIncidentId);
    } finally {
      postChecking = false;
    }
  }

  function postCheckStatus(nodeId) {
    return postCheckResults.find(r => r.stepNodeId === nodeId);
  }
</script>

<div class="plan-view">
  <div class="plan-header">
    <button class="back-btn" on:click={onBack}>&larr; Back</button>
    <h2>Fix Plan</h2>
    {#if plan}
      <span class="ns-label">{plan.namespace}</span>
    {/if}
  </div>

  <div class="plan-body">
    {#if loading}
      <div class="loading">Diagnosing cluster state...</div>
    {:else if error}
      <div class="error-msg">{error}</div>
    {:else if plan && plan.steps}
      {#each plan.steps as step}
        <div class="step" class:root-cause={step.isRootCause} class:auto-resolve={step.willAutoResolve}>
          <div class="step-header">
            <span class="step-num">{step.order}</span>
            <div class="step-badge" class:badge-root={step.isRootCause} class:badge-auto={step.willAutoResolve}>
              {step.isRootCause ? 'FIX THIS' : step.willAutoResolve ? 'AUTO-RESOLVES' : 'ACTION NEEDED'}
            </div>
            <div class="step-resource">
              <span class="step-kind">{step.resourceKind}</span>
              {step.resourceName}
            </div>
          </div>

          <!-- Error codes -->
          {#if step.errorCodes?.length > 0}
            <div class="step-codes">
              {#each step.errorCodes as code}
                <span class="code-tag">{code}</span>
              {/each}
            </div>
          {/if}

          <!-- Remediation -->
          <div class="step-remediation">{step.remediation}</div>

          <!-- Warnings -->
          {#if step.warnings?.length > 0}
            <div class="step-warnings">
              {#each step.warnings as warning}
                <div class="warning-item">{warning}</div>
              {/each}
            </div>
          {/if}

          <!-- KB Insights -->
          {#if step.kbInsights?.length > 0}
            <div class="step-insights">
              {#each step.kbInsights as insight}
                <div class="insight-item">{insight}</div>
              {/each}
            </div>
          {/if}

          <!-- Past Attempts -->
          {#if step.pastAttempts?.length > 0}
            <details class="past-attempts">
              <summary>Past attempts ({step.pastAttempts.length})</summary>
              {#each step.pastAttempts as attempt}
                <div class="attempt" class:attempt-ok={attempt.result === 'success'} class:attempt-fail={attempt.result === 'failure'}>
                  <span class="attempt-result">{attempt.result}</span>
                  <span class="attempt-when">{attempt.when}</span>
                  {#if attempt.treePath}<span class="attempt-path">{attempt.treePath}</span>{/if}
                  {#if attempt.errorMessage}<span class="attempt-err">{attempt.errorMessage}</span>{/if}
                </div>
              {/each}
            </details>
          {/if}

          <!-- Fix Options (from decision engine) -->
          {#if step.options?.length > 0}
            <div class="options-section">
              <div class="options-label">Fix options</div>
              {#each step.options as option, optIdx}
                <details class="option-card" class:option-low={option.risk === 'low'} class:option-medium={option.risk === 'medium'} class:option-high={option.risk === 'high'}>
                  <summary>
                    <span class="option-risk risk-{option.risk}">{option.risk}</span>
                    {option.label}
                  </summary>
                  <div class="option-body">
                    <p class="option-desc">{option.description}</p>
                    {#if option.warnings?.length > 0}
                      <div class="option-warnings">
                        {#each option.warnings as w}
                          <div class="warning-item">{w}</div>
                        {/each}
                      </div>
                    {/if}
                    {#if option.commands?.length > 0}
                      <div class="cmd-list">
                        {#each option.commands as cmd}
                          <div class="cmd-item">
                            <div class="cmd-label">
                              {cmd.label}
                              {#if cmd.destructive}<span class="destructive-tag">modifies cluster</span>{/if}
                            </div>
                            <div class="cmd-row">
                              <code class="cmd-text">{cmd.command}</code>
                              <button class="copy-btn" on:click={() => copyCmd(cmd.command)} title="Copy">&#x2398;</button>
                              <button
                                class="run-btn"
                                class:destructive={cmd.destructive}
                                on:click={() => runCmd(cmd.command, stepErrorCodes(step), step, cmd.destructive)}
                                disabled={runningCmds[cmd.command]}
                              >
                                {#if runningCmds[cmd.command]}...{:else}&#x25B6;{/if}
                              </button>
                            </div>
                            {#if cmdResults[cmd.command]}
                              {#if cmdResults[cmd.command].suggestion?.insight}
                                <div class="insight">{cmdResults[cmd.command].suggestion.insight}</div>
                              {/if}
                              <details class="cmd-output">
                                <summary>Output ({cmdResults[cmd.command].success ? 'ok' : 'error'})</summary>
                                <div class="cmd-result" class:cmd-error={!cmdResults[cmd.command].success}>
                                  <pre>{cmdResults[cmd.command].output || cmdResults[cmd.command].error || 'No output'}</pre>
                                </div>
                              </details>
                            {/if}
                          </div>
                        {/each}
                      </div>
                    {:else if !option.commands || option.commands.length === 0}
                      <p class="option-skip">No action needed</p>
                    {/if}
                    {#if option.rollback?.length > 0}
                      <details class="rollback-section">
                        <summary>Rollback</summary>
                        {#each option.rollback as rb}
                          <div class="cmd-item">
                            <div class="cmd-label">{rb.label}</div>
                            <div class="cmd-row">
                              <code class="cmd-text">{rb.command}</code>
                              <button class="copy-btn" on:click={() => copyCmd(rb.command)} title="Copy">&#x2398;</button>
                              <button class="run-btn destructive" on:click={() => runCmd(rb.command, stepErrorCodes(step), step, true)} disabled={runningCmds[rb.command]}>
                                {#if runningCmds[rb.command]}...{:else}&#x25B6;{/if}
                              </button>
                            </div>
                          </div>
                        {/each}
                      </details>
                    {/if}
                  </div>
                </details>
              {/each}
            </div>
          {/if}

          <!-- Legacy commands (when no options from decision engine) -->
          {#if (!step.options || step.options.length === 0) && step.commands?.length > 0}
            <div class="cmd-list">
              {#each step.commands as cmd}
                <div class="cmd-item">
                  <div class="cmd-label">
                    {cmd.label}
                    {#if cmd.destructive}<span class="destructive-tag">modifies cluster</span>{/if}
                  </div>
                  <div class="cmd-row">
                    <code class="cmd-text">{cmd.command}</code>
                    <button class="copy-btn" on:click={() => copyCmd(cmd.command)} title="Copy">&#x2398;</button>
                    <button
                      class="run-btn"
                      class:destructive={cmd.destructive}
                      on:click={() => runCmd(cmd.command, stepErrorCodes(step), step, cmd.destructive)}
                      disabled={runningCmds[cmd.command]}
                      title={cmd.destructive ? 'Apply (modifies cluster)' : 'Run'}
                    >
                      {#if runningCmds[cmd.command]}...{:else}&#x25B6;{/if}
                    </button>
                  </div>
                  {#if cmdResults[cmd.command]}
                    {#if cmdResults[cmd.command].suggestion?.insight}
                      <div class="insight">{cmdResults[cmd.command].suggestion.insight}</div>
                    {/if}
                    <details class="cmd-output">
                      <summary>Output ({cmdResults[cmd.command].success ? 'ok' : 'error'})</summary>
                      <div class="cmd-result" class:cmd-error={!cmdResults[cmd.command].success}>
                        <pre>{cmdResults[cmd.command].output || cmdResults[cmd.command].error || 'No output'}</pre>
                      </div>
                    </details>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}

          <!-- Dependencies -->
          {#if step.dependsOn?.length > 0}
            <div class="step-deps">
              Depends on: {step.dependsOn.map(d => d.split('/').pop()).join(', ')}
            </div>
          {/if}

          <!-- Post-check result -->
          {#if postCheckStatus(step.nodeId)}
            {@const pc = postCheckStatus(step.nodeId)}
            <div class="post-check" class:pc-fixed={pc.status === 'fixed'} class:pc-improved={pc.status === 'improved'} class:pc-unchanged={pc.status === 'unchanged'} class:pc-worse={pc.status === 'worse'}>
              <span class="pc-status">{pc.status}</span>
              <span class="pc-detail">{pc.errorsBefore} errors &rarr; {pc.errorsAfter}</span>
              {#if pc.resolved?.length > 0}<span class="pc-resolved">Resolved: {pc.resolved.join(', ')}</span>{/if}
              {#if pc.newIssues?.length > 0}<span class="pc-new">New: {pc.newIssues.join(', ')}</span>{/if}
            </div>
          {/if}

          <!-- Diagnostic findings -->
          {#if step.diagnostics?.length > 0}
            <details class="step-diags">
              <summary>Diagnostic findings ({step.diagnostics.reduce((n, d) => n + (d.findings?.length || 0), 0)})</summary>
              {#each step.diagnostics as diag}
                {#each diag.findings || [] as finding}
                  <div class="finding">
                    <span class="finding-cat">{finding.category}</span>
                    <span class="finding-summary">{finding.summary}</span>
                  </div>
                {/each}
              {/each}
            </details>
          {/if}
        </div>
      {/each}

      <!-- Dynamically suggested follow-up commands -->
      {#if suggestedCmds.length > 0}
        <div class="suggested-section">
          <h3>Suggested next steps</h3>
          <div class="cmd-list">
            {#each suggestedCmds as cmd}
              <div class="cmd-item">
                <div class="cmd-label">
                  {cmd.label}
                  {#if cmd.destructive}
                    <span class="destructive-tag">modifies cluster</span>
                  {/if}
                </div>
                <div class="cmd-row">
                  <code class="cmd-text">{cmd.command}</code>
                  <button class="copy-btn" on:click={() => copyCmd(cmd.command)} title="Copy">&#x2398;</button>
                  <button
                    class="run-btn"
                    class:destructive={cmd.destructive}
                    on:click={() => runCmd(cmd.command, [])}
                    disabled={runningCmds[cmd.command]}
                  >
                    {#if runningCmds[cmd.command]}
                      ...
                    {:else}
                      &#x25B6;
                    {/if}
                  </button>
                </div>
                {#if cmdResults[cmd.command]}
                  {#if cmdResults[cmd.command].suggestion?.insight}
                    <div class="insight">
                      {cmdResults[cmd.command].suggestion.insight}
                    </div>
                  {/if}
                  <details class="cmd-output">
                    <summary>Output ({cmdResults[cmd.command].success ? 'ok' : 'error'})</summary>
                    <div class="cmd-result" class:cmd-error={!cmdResults[cmd.command].success}>
                      <pre>{cmdResults[cmd.command].output || cmdResults[cmd.command].error || 'No output'}</pre>
                    </div>
                  </details>
                {/if}
              </div>
            {/each}
          </div>
        </div>
      {/if}

      <div class="plan-footer">
        <button class="postcheck-btn" disabled={postChecking} on:click={runPostCheck}>
          {postChecking ? 'Checking...' : 'Post-check (verify fixes)'}
        </button>
        <button class="refresh-btn" disabled={loading} on:click={rescanAndDiagnose}>
          {loading ? 'Scanning & diagnosing...' : 'Re-scan & diagnose'}
        </button>
      </div>
    {:else}
      <div class="empty">No fix plan available</div>
    {/if}
  </div>
</div>

<style>
  .plan-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .plan-header {
    padding: 16px 24px;
    display: flex;
    align-items: center;
    gap: 12px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }
  .plan-header h2 {
    font-size: 18px;
    font-weight: 700;
  }
  .back-btn {
    font-size: 13px;
    padding: 6px 14px;
    border-radius: 6px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    color: var(--text-primary);
  }
  .back-btn:hover { background: var(--bg-tertiary); }
  .ns-label {
    font-size: 12px;
    font-weight: 600;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-left: auto;
  }
  .plan-body {
    flex: 1;
    overflow-y: auto;
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .loading, .error-msg, .empty {
    color: var(--text-muted);
    font-size: 14px;
    text-align: center;
    padding: 40px 0;
  }
  .error-msg { color: var(--red, #ef4444); }

  .step {
    padding: 16px;
    border-radius: 8px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-left: 4px solid var(--border);
  }
  .step.root-cause {
    border-left-color: var(--red, #ef4444);
  }
  .step.auto-resolve {
    border-left-color: var(--text-muted);
    opacity: 0.7;
  }

  .step-header {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 8px;
  }
  .step-num {
    width: 24px;
    height: 24px;
    border-radius: 50%;
    background: var(--bg-tertiary);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    font-weight: 700;
    flex-shrink: 0;
  }
  .step-badge {
    font-size: 10px;
    padding: 2px 8px;
    border-radius: 4px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    flex-shrink: 0;
  }
  .badge-root {
    background: var(--red, #ef4444);
    color: white;
  }
  .badge-auto {
    background: var(--bg-tertiary);
    color: var(--text-muted);
  }
  .step-resource {
    font-size: 14px;
    font-weight: 600;
  }
  .step-kind {
    color: var(--text-muted);
    font-weight: 400;
    margin-right: 4px;
  }

  .step-codes {
    display: flex;
    gap: 6px;
    margin-bottom: 8px;
    flex-wrap: wrap;
  }
  .code-tag {
    font-size: 11px;
    padding: 1px 6px;
    border-radius: 4px;
    background: var(--bg-tertiary);
    color: var(--text-secondary);
  }

  .step-remediation {
    font-size: 13px;
    color: var(--text-primary);
    line-height: 1.5;
    margin-bottom: 8px;
  }

  .step-deps {
    font-size: 12px;
    color: var(--text-muted);
    margin-bottom: 8px;
  }

  .cmd-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-bottom: 10px;
  }
  .cmd-label {
    font-size: 12px;
    color: var(--text-secondary);
    margin-bottom: 3px;
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .destructive-tag {
    font-size: 10px;
    padding: 1px 5px;
    border-radius: 3px;
    background: var(--red, #ef4444);
    color: white;
    font-weight: 600;
  }
  .cmd-row {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .cmd-text {
    flex: 1;
    font-size: 12px;
    padding: 6px 10px;
    background: var(--bg-tertiary, #27272a);
    border-radius: 4px;
    color: var(--text-primary);
    overflow-x: auto;
    white-space: nowrap;
  }
  .copy-btn, .run-btn {
    flex-shrink: 0;
    font-size: 14px;
    padding: 4px 8px;
    border-radius: 4px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    color: var(--text-muted);
    cursor: pointer;
  }
  .copy-btn:hover, .run-btn:hover {
    color: var(--text-primary);
    background: var(--bg-tertiary);
  }
  .run-btn.destructive {
    border-color: var(--red, #ef4444);
    color: var(--red, #ef4444);
  }
  .run-btn.destructive:hover {
    background: var(--red, #ef4444);
    color: white;
  }
  .run-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .cmd-result {
    margin-top: 6px;
    padding: 8px 10px;
    border-radius: 4px;
    background: var(--bg-tertiary, #27272a);
    border: 1px solid var(--border);
    max-height: 200px;
    overflow: auto;
  }
  .cmd-result pre {
    font-size: 11px;
    color: var(--text-secondary);
    white-space: pre-wrap;
    word-break: break-all;
    margin: 0;
  }
  .cmd-result.cmd-error {
    border-color: var(--red, #ef4444);
  }
  .cmd-result.cmd-error pre {
    color: var(--red, #ef4444);
  }

  .insight {
    margin-top: 6px;
    padding: 8px 12px;
    border-radius: 4px;
    background: color-mix(in srgb, var(--accent) 15%, transparent);
    border-left: 3px solid var(--accent);
    font-size: 12px;
    color: var(--text-primary);
    line-height: 1.4;
  }
  .cmd-output {
    margin-top: 4px;
  }
  .cmd-output summary {
    font-size: 11px;
    color: var(--text-muted);
    cursor: pointer;
  }
  .suggested-section {
    padding: 16px;
    border-radius: 8px;
    background: var(--bg-secondary);
    border: 1px solid var(--accent);
    border-left: 4px solid var(--accent);
  }
  .suggested-section h3 {
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--accent);
    margin-bottom: 12px;
  }

  .step-diags {
    margin-top: 8px;
  }
  .step-diags summary {
    font-size: 12px;
    color: var(--accent);
    cursor: pointer;
    margin-bottom: 6px;
  }
  .finding {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    padding: 4px 0;
    font-size: 12px;
  }
  .finding-cat {
    font-size: 10px;
    padding: 1px 5px;
    border-radius: 3px;
    background: var(--bg-tertiary);
    color: var(--text-muted);
    flex-shrink: 0;
    text-transform: uppercase;
  }
  .finding-summary {
    color: var(--text-secondary);
    line-height: 1.4;
  }

  .plan-footer {
    padding-top: 12px;
    border-top: 1px solid var(--border);
  }
  .refresh-btn {
    width: 100%;
    padding: 8px;
    border-radius: 6px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    color: var(--text-primary);
    font-size: 13px;
  }
  .refresh-btn:hover { background: var(--bg-tertiary); }

  /* Warnings */
  .step-warnings { margin-bottom: 8px; }
  .warning-item {
    font-size: 12px;
    padding: 6px 10px;
    border-radius: 4px;
    background: color-mix(in srgb, #f59e0b 12%, transparent);
    border-left: 3px solid #f59e0b;
    color: #f59e0b;
    margin-bottom: 4px;
    line-height: 1.4;
  }

  /* KB Insights */
  .step-insights { margin-bottom: 8px; }
  .insight-item {
    font-size: 12px;
    padding: 6px 10px;
    border-radius: 4px;
    background: color-mix(in srgb, var(--accent) 10%, transparent);
    border-left: 3px solid var(--accent);
    color: var(--text-secondary);
    margin-bottom: 4px;
    line-height: 1.4;
  }

  /* Past Attempts */
  .past-attempts { margin-bottom: 8px; }
  .past-attempts summary {
    font-size: 12px;
    color: var(--text-muted);
    cursor: pointer;
    margin-bottom: 4px;
  }
  .attempt {
    display: flex;
    gap: 8px;
    align-items: center;
    font-size: 11px;
    padding: 4px 8px;
    border-radius: 4px;
    margin-bottom: 2px;
  }
  .attempt-ok { background: color-mix(in srgb, #22c55e 10%, transparent); }
  .attempt-fail { background: color-mix(in srgb, #ef4444 10%, transparent); }
  .attempt-result {
    font-weight: 700;
    text-transform: uppercase;
    font-size: 10px;
  }
  .attempt-ok .attempt-result { color: #22c55e; }
  .attempt-fail .attempt-result { color: #ef4444; }
  .attempt-when { color: var(--text-muted); }
  .attempt-path { color: var(--text-secondary); font-family: monospace; font-size: 10px; }
  .attempt-err { color: var(--red, #ef4444); font-size: 10px; }

  /* Fix Options */
  .options-section { margin-bottom: 10px; }
  .options-label {
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-muted);
    margin-bottom: 6px;
  }
  .option-card {
    border: 1px solid var(--border);
    border-radius: 6px;
    margin-bottom: 6px;
    overflow: hidden;
  }
  .option-card summary {
    padding: 8px 12px;
    font-size: 13px;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--bg-tertiary);
  }
  .option-card summary:hover { background: var(--bg-secondary); }
  .option-body {
    padding: 10px 12px;
  }
  .option-desc {
    font-size: 12px;
    color: var(--text-secondary);
    line-height: 1.5;
    margin: 0 0 8px 0;
  }
  .option-skip {
    font-size: 12px;
    color: var(--text-muted);
    font-style: italic;
    margin: 0;
  }
  .option-warnings { margin-bottom: 8px; }
  .option-risk {
    font-size: 10px;
    font-weight: 700;
    padding: 1px 6px;
    border-radius: 3px;
    text-transform: uppercase;
    flex-shrink: 0;
  }
  .risk-low { background: color-mix(in srgb, #22c55e 20%, transparent); color: #22c55e; }
  .risk-medium { background: color-mix(in srgb, #f59e0b 20%, transparent); color: #f59e0b; }
  .risk-high { background: color-mix(in srgb, #ef4444 20%, transparent); color: #ef4444; }

  .rollback-section { margin-top: 8px; }
  .rollback-section summary {
    font-size: 11px;
    color: var(--text-muted);
    cursor: pointer;
  }

  /* Post-check */
  .post-check {
    display: flex;
    gap: 8px;
    align-items: center;
    font-size: 12px;
    padding: 8px 12px;
    border-radius: 6px;
    margin-top: 8px;
    border: 1px solid var(--border);
  }
  .pc-fixed { background: color-mix(in srgb, #22c55e 12%, transparent); border-color: #22c55e; }
  .pc-improved { background: color-mix(in srgb, #3b82f6 12%, transparent); border-color: #3b82f6; }
  .pc-unchanged { background: var(--bg-tertiary); }
  .pc-worse { background: color-mix(in srgb, #ef4444 12%, transparent); border-color: #ef4444; }
  .pc-status {
    font-weight: 700;
    text-transform: uppercase;
    font-size: 10px;
    letter-spacing: 0.05em;
  }
  .pc-fixed .pc-status { color: #22c55e; }
  .pc-improved .pc-status { color: #3b82f6; }
  .pc-unchanged .pc-status { color: var(--text-muted); }
  .pc-worse .pc-status { color: #ef4444; }
  .pc-detail { color: var(--text-secondary); }
  .pc-resolved { color: #22c55e; font-size: 11px; }
  .pc-new { color: #ef4444; font-size: 11px; }

  .postcheck-btn {
    width: 100%;
    padding: 8px;
    border-radius: 6px;
    background: color-mix(in srgb, var(--accent) 15%, transparent);
    border: 1px solid var(--accent);
    color: var(--accent);
    font-size: 13px;
    margin-bottom: 8px;
  }
  .postcheck-btn:hover { background: color-mix(in srgb, var(--accent) 25%, transparent); }
  .postcheck-btn:disabled { opacity: 0.5; }

  .plan-footer {
    display: flex;
    flex-direction: column;
    gap: 0;
  }
</style>
