<script>
  export let incident;
  export let selected = false;

  $: severityClass = `badge badge-${incident.severity}`;
  $: categoryClass = `badge badge-${incident.category}`;
  $: errorCount = incident.errors?.length || 0;
  $: codeCount = new Set(incident.errorCodes || []).size;
  $: nodeCount = incident.affectedNodes?.length || 0;
</script>

<button
  class="card"
  class:selected
  on:click
>
  <div class="card-left">
    <div class="card-header">
      <span class={severityClass}>{incident.severity}</span>
      <span class={categoryClass}>{incident.category}</span>
      {#if incident.namespace}
        <span class="ns">{incident.namespace}</span>
      {/if}
      <span class="incident-id">{incident.id}</span>
    </div>
    <div class="card-summary">{incident.summary}</div>
  </div>
  <div class="card-right">
    <div class="stat">
      <span class="stat-num">{errorCount}</span>
      <span class="stat-label">{errorCount === 1 ? 'issue' : 'issues'}</span>
    </div>
    <div class="stat">
      <span class="stat-num">{nodeCount}</span>
      <span class="stat-label">affected</span>
    </div>
    <span class="arrow">&rsaquo;</span>
  </div>
</button>

<style>
  .card {
    width: 100%;
    padding: 12px 16px;
    border-radius: 8px;
    text-align: left;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    transition: border-color 0.15s;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
  }
  .card:hover { border-color: var(--accent); }
  .card.selected { border-color: var(--accent); box-shadow: 0 0 0 1px var(--accent); }
  .card-left { flex: 1; min-width: 0; }
  .card-header {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-bottom: 6px;
    flex-wrap: wrap;
  }
  .incident-id { color: var(--text-muted); font-size: 11px; }
  .ns { color: var(--accent); font-size: 11px; }
  .card-summary {
    font-size: 13px;
    line-height: 1.4;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
  }
  .card-right {
    display: flex;
    align-items: center;
    gap: 16px;
    flex-shrink: 0;
  }
  .stat {
    display: flex;
    flex-direction: column;
    align-items: center;
    min-width: 48px;
  }
  .stat-num { font-size: 18px; font-weight: 700; color: var(--text-primary); }
  .stat-label { font-size: 10px; color: var(--text-muted); text-transform: uppercase; }
  .arrow {
    font-size: 24px;
    color: var(--text-muted);
    line-height: 1;
  }
</style>
