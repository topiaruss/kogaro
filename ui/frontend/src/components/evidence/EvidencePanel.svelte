<script>
  import { selectedNodeId, faultGraph } from '../../lib/stores/graphStore';
  import { fetchNodeDetail } from '../../lib/api/wailsBridge';

  let detail = null;
  let loading = false;

  $: if ($selectedNodeId) {
    loadDetail($selectedNodeId);
  } else {
    detail = null;
  }

  async function loadDetail(nodeId) {
    loading = true;
    detail = await fetchNodeDetail(nodeId);
    loading = false;
  }

  function close() {
    selectedNodeId.set(null);
  }

  function severityClass(s) {
    return `badge badge-${s}`;
  }
</script>

<div class="panel">
  <div class="panel-header">
    <h3>Evidence</h3>
    <button class="close-btn" on:click={close}>x</button>
  </div>

  {#if loading}
    <div class="loading">Loading...</div>
  {:else if detail}
    <div class="section">
      <div class="resource-header">
        <span class="kind">{detail.node.kind}</span>
        <span class="health-dot {detail.node.health}"></span>
      </div>
      <div class="resource-name">{detail.node.name}</div>
      {#if detail.node.namespace}
        <div class="resource-ns">ns: {detail.node.namespace}</div>
      {/if}
    </div>

    {#if detail.node.errorCodes?.length}
      <div class="section">
        <h4>Error Codes</h4>
        <div class="codes">
          {#each detail.node.errorCodes as code}
            <span class="badge badge-error">{code}</span>
          {/each}
        </div>
      </div>
    {/if}

    {#if detail.errors?.length}
      <div class="section">
        <h4>Errors ({detail.errors.length})</h4>
        {#each detail.errors as err}
          <div class="error-item">
            <div class="error-header">
              <span class={severityClass(err.severity)}>{err.errorCode}</span>
            </div>
            <div class="error-msg">{err.message}</div>
            {#if err.remediationHint}
              <div class="hint">{err.remediationHint}</div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}

    {#if detail.incomingEdges?.length}
      <div class="section">
        <h4>Incoming ({detail.incomingEdges.length})</h4>
        {#each detail.incomingEdges as edge}
          <div class="edge-item">
            <span class="edge-source">{edge.source}</span>
            <span class="edge-label">{edge.label || edge.type}</span>
          </div>
        {/each}
      </div>
    {/if}

    {#if detail.outgoingEdges?.length}
      <div class="section">
        <h4>Outgoing ({detail.outgoingEdges.length})</h4>
        {#each detail.outgoingEdges as edge}
          <div class="edge-item">
            <span class="edge-label">{edge.label || edge.type}</span>
            <span class="edge-target">{edge.target}</span>
          </div>
        {/each}
      </div>
    {/if}

    {#if detail.node.labels && Object.keys(detail.node.labels).length}
      <div class="section">
        <h4>Labels</h4>
        {#each Object.entries(detail.node.labels) as [k, v]}
          <div class="label-item"><span class="label-key">{k}:</span> {v}</div>
        {/each}
      </div>
    {/if}

    {#if detail.node.details && Object.keys(detail.node.details).length}
      <div class="section">
        <h4>Details</h4>
        {#each Object.entries(detail.node.details) as [k, v]}
          <div class="label-item"><span class="label-key">{k}:</span> {v}</div>
        {/each}
      </div>
    {/if}
  {:else}
    <div class="loading">No details available</div>
  {/if}
</div>

<style>
  .panel { padding: 16px; }
  .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
  }
  .panel-header h3 { font-size: 14px; font-weight: 600; }
  .close-btn {
    width: 24px; height: 24px; padding: 0;
    display: flex; align-items: center; justify-content: center;
    font-size: 14px; border-radius: 4px;
  }
  .section {
    margin-bottom: 16px;
    padding-bottom: 16px;
    border-bottom: 1px solid var(--border);
  }
  .section:last-child { border-bottom: none; }
  h4 { font-size: 11px; text-transform: uppercase; color: var(--text-muted); margin-bottom: 8px; letter-spacing: 0.05em; }
  .resource-header { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
  .kind { font-size: 11px; color: var(--text-muted); text-transform: uppercase; }
  .resource-name { font-size: 16px; font-weight: 600; word-break: break-all; }
  .resource-ns { font-size: 12px; color: var(--accent); margin-top: 2px; }
  .health-dot {
    width: 8px; height: 8px; border-radius: 50%;
    display: inline-block;
  }
  .health-dot.broken { background: var(--red); }
  .health-dot.missing { background: var(--red); }
  .health-dot.degraded { background: var(--orange); }
  .health-dot.healthy { background: var(--green); }
  .health-dot.unknown { background: var(--text-muted); }
  .codes { display: flex; flex-wrap: wrap; gap: 4px; }
  .error-item {
    margin-bottom: 10px;
    padding: 8px;
    border-radius: 6px;
    background: var(--bg-primary);
  }
  .error-header { margin-bottom: 4px; }
  .error-msg { font-size: 13px; line-height: 1.4; }
  .hint {
    margin-top: 4px;
    font-size: 12px;
    color: var(--green);
    font-style: italic;
  }
  .edge-item {
    display: flex; gap: 6px; font-size: 12px;
    padding: 4px 0; color: var(--text-secondary);
  }
  .edge-label { color: var(--text-muted); }
  .edge-source, .edge-target { font-family: monospace; font-size: 11px; }
  .label-item { font-size: 12px; padding: 2px 0; color: var(--text-secondary); }
  .label-key { color: var(--text-muted); }
  .loading { color: var(--text-muted); font-size: 13px; padding: 20px 0; text-align: center; }
</style>
