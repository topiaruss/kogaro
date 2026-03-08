<script>
  import { selectedIncidentId } from '../../lib/stores/graphStore';
  import { fetchFixPlan, runCommand, runScan } from '../../lib/api/wailsBridge';

  export let onBack = () => {};

  let plan = null;
  let loading = false;
  let error = null;
  let cmdResults = {}; // keyed by command string
  let runningCmds = {}; // keyed by command string
  let suggestedCmds = []; // dynamically suggested commands from analysis

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

  async function runCmd(cmd, errorCodes) {
    runningCmds[cmd] = true;
    runningCmds = runningCmds;
    try {
      const result = await runCommand(cmd, errorCodes || []);
      cmdResults[cmd] = result;
      cmdResults = cmdResults;

      // Add suggested next commands from analysis
      if (result.suggestion?.nextCmds?.length > 0) {
        for (const next of result.suggestion.nextCmds) {
          // Avoid duplicates
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

          <!-- Commands -->
          {#if step.commands?.length > 0}
            <div class="cmd-list">
              {#each step.commands as cmd}
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
                      on:click={() => runCmd(cmd.command, stepErrorCodes(step))}
                      disabled={runningCmds[cmd.command]}
                      title={cmd.destructive ? 'Apply (modifies cluster)' : 'Run'}
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
          {/if}

          <!-- Dependencies -->
          {#if step.dependsOn?.length > 0}
            <div class="step-deps">
              Depends on: {step.dependsOn.map(d => d.split('/').pop()).join(', ')}
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
</style>
