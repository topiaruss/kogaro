<script>
  import { selectedIncidentId, faultGraph } from '../../lib/stores/graphStore';

  export let onViewGraph = () => {};
  export let onDiagnose = () => {};

  $: incident = $faultGraph?.incidents?.find(i => i.id === $selectedIncidentId) || null;

  // Deduplicate errors by code + resource
  $: uniqueErrors = dedup(incident?.errors || []);

  function dedup(errors) {
    const seen = new Set();
    return errors.filter(e => {
      const key = `${e.errorCode}/${e.namespace}/${e.resourceName}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    });
  }

  // Group errors by resource for the "what's wrong" section
  $: errorsByResource = groupByResource(uniqueErrors);

  function groupByResource(errors) {
    const groups = new Map();
    for (const e of errors) {
      const key = `${e.resourceType}/${e.namespace}/${e.resourceName}`;
      if (!groups.has(key)) groups.set(key, { type: e.resourceType, name: e.resourceName, ns: e.namespace, errors: [] });
      groups.get(key).errors.push(e);
    }
    return [...groups.values()];
  }

  // Generate kubectl commands from error context
  function kubectlCommands(incident) {
    if (!incident) return [];
    const cmds = [];
    const ns = incident.namespace;
    const seen = new Set();

    for (const e of uniqueErrors) {
      // Resource-specific commands
      const describeCmd = `kubectl describe ${e.resourceType.toLowerCase()} ${e.resourceName} -n ${e.namespace}`;
      if (!seen.has(describeCmd)) {
        seen.add(describeCmd);
        cmds.push({ label: `Describe ${e.resourceType} ${e.resourceName}`, cmd: describeCmd });
      }

      // Category-specific commands
      if (e.errorCode.includes('NET-002') || e.errorCode.includes('NET-009')) {
        const endpointsCmd = `kubectl get endpoints ${e.resourceName} -n ${e.namespace}`;
        if (!seen.has(endpointsCmd)) {
          seen.add(endpointsCmd);
          cmds.push({ label: `Check endpoints`, cmd: endpointsCmd });
        }
        const podsCmd = `kubectl get pods -n ${e.namespace} -o wide`;
        if (!seen.has(podsCmd)) {
          seen.add(podsCmd);
          cmds.push({ label: `List pods`, cmd: podsCmd });
        }
      }

      if (e.errorCode.includes('NET-001') || e.errorCode.includes('NET-002')) {
        const svcCmd = `kubectl get svc ${e.resourceName} -n ${e.namespace} -o yaml`;
        if (!seen.has(svcCmd)) {
          seen.add(svcCmd);
          cmds.push({ label: `Show service selector`, cmd: svcCmd });
        }
      }
    }

    // Limit to most useful
    return cmds.slice(0, 5);
  }

  $: commands = kubectlCommands(incident);

  function copyCmd(cmd) {
    navigator.clipboard.writeText(cmd);
  }

  function severityIcon(severity) {
    if (severity === 'error') return '●';
    if (severity === 'warning') return '▲';
    return '○';
  }

  function severityColor(severity) {
    if (severity === 'error') return 'var(--red, #ef4444)';
    if (severity === 'warning') return 'var(--orange, #f97316)';
    return 'var(--text-muted)';
  }
</script>

{#if incident}
<div class="detail-view">
  <div class="detail-header">
    <button class="back-btn" on:click={() => selectedIncidentId.set(null)}>
      &larr; Back
    </button>
    <div class="header-right">
      <span class="badge badge-{incident.severity}">{incident.severity}</span>
      <span class="badge badge-{incident.category}">{incident.category}</span>
      <span class="incident-id">{incident.id}</span>
    </div>
  </div>

  <div class="detail-body">
    <!-- Summary -->
    <h2 class="summary">{incident.summary}</h2>
    <div class="meta">
      <span class="ns-label">{incident.namespace}</span>
      <span class="counts">{uniqueErrors.length} {uniqueErrors.length === 1 ? 'issue' : 'issues'} · {incident.affectedNodes?.length || 0} affected</span>
    </div>

    <!-- What's Wrong -->
    <section class="section">
      <h3>What's wrong</h3>
      {#each errorsByResource as group}
        <div class="resource-group">
          <div class="resource-name">
            <span class="resource-kind">{group.type}</span>
            {group.name}
          </div>
          {#each group.errors as error}
            <div class="error-item">
              <span class="severity-dot" style="color: {severityColor(error.severity)}">{severityIcon(error.severity)}</span>
              <div class="error-content">
                <div class="error-msg">{error.message}</div>
                <span class="error-code">{error.errorCode}</span>
              </div>
            </div>
          {/each}
        </div>
      {/each}
    </section>

    <!-- How to Fix -->
    {#if uniqueErrors.some(e => e.remediationHint)}
    <section class="section">
      <h3>How to fix</h3>
      <ol class="fix-list">
        {#each uniqueErrors.filter(e => e.remediationHint) as error}
          <li>{error.remediationHint}</li>
        {/each}
      </ol>
    </section>
    {/if}

    <!-- Quick Commands -->
    {#if commands.length > 0}
    <section class="section">
      <h3>Investigate</h3>
      <div class="cmd-list">
        {#each commands as { label, cmd }}
          <div class="cmd-item">
            <div class="cmd-label">{label}</div>
            <div class="cmd-row">
              <code class="cmd-text">{cmd}</code>
              <button class="copy-btn" on:click={() => copyCmd(cmd)} title="Copy">⎘</button>
            </div>
          </div>
        {/each}
      </div>
    </section>
    {/if}

    <!-- Actions -->
    <div class="actions">
      <button class="action-btn primary" on:click={onDiagnose}>
        Diagnose &amp; Fix Plan
      </button>
      <button class="action-btn secondary" on:click={onViewGraph}>
        View dependency graph &rarr;
      </button>
    </div>
  </div>
</div>
{/if}

<style>
  .detail-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .detail-header {
    padding: 16px 24px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
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
  .header-right {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .incident-id { color: var(--text-muted); font-size: 12px; }
  .detail-body {
    flex: 1;
    overflow-y: auto;
    padding: 24px;
  }
  .summary {
    font-size: 18px;
    font-weight: 600;
    line-height: 1.4;
    margin-bottom: 8px;
  }
  .meta {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 24px;
  }
  .ns-label {
    font-size: 12px;
    font-weight: 600;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .counts {
    font-size: 13px;
    color: var(--text-muted);
  }
  .section {
    margin-bottom: 24px;
  }
  h3 {
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-muted);
    margin-bottom: 12px;
  }
  .resource-group {
    margin-bottom: 12px;
  }
  .resource-name {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary);
    margin-bottom: 6px;
  }
  .resource-kind {
    color: var(--text-muted);
    font-weight: 400;
    margin-right: 4px;
  }
  .error-item {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    padding: 6px 0 6px 8px;
  }
  .severity-dot {
    flex-shrink: 0;
    font-size: 10px;
    line-height: 1.6;
  }
  .error-content {
    flex: 1;
    min-width: 0;
  }
  .error-msg {
    font-size: 13px;
    color: var(--text-primary);
    line-height: 1.4;
  }
  .error-code {
    font-size: 11px;
    color: var(--text-muted);
  }
  .fix-list {
    padding-left: 20px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .fix-list li {
    font-size: 13px;
    color: var(--text-primary);
    line-height: 1.5;
  }
  .cmd-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .cmd-item {}
  .cmd-label {
    font-size: 12px;
    color: var(--text-secondary);
    margin-bottom: 3px;
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
  .copy-btn {
    flex-shrink: 0;
    font-size: 14px;
    padding: 4px 8px;
    border-radius: 4px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    color: var(--text-muted);
    cursor: pointer;
  }
  .copy-btn:hover {
    color: var(--text-primary);
    background: var(--bg-tertiary);
  }
  .actions {
    padding-top: 12px;
    border-top: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .action-btn {
    width: 100%;
    padding: 10px;
    border-radius: 8px;
    font-size: 14px;
    font-weight: 600;
    cursor: pointer;
    border: none;
  }
  .action-btn.primary {
    background: var(--accent);
    color: white;
  }
  .action-btn.primary:hover { opacity: 0.9; }
  .action-btn.secondary {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    color: var(--text-primary);
  }
  .action-btn.secondary:hover { background: var(--bg-tertiary); }
</style>
