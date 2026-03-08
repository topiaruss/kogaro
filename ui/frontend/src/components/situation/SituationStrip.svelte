<script>
  import { incidents, selectedIncidentId, faultGraph, severityFilter } from '../../lib/stores/graphStore';
  import IncidentCard from './IncidentCard.svelte';

  function selectIncident(id) {
    selectedIncidentId.set(id);
  }

  $: errorCount = $incidents.filter(i => i.severity === 'error').length;
  $: warningCount = ($faultGraph?.incidents ?? []).filter(i => i.severity === 'warning').length;
  $: totalCount = $faultGraph?.incidents?.length ?? 0;

  // Group incidents by namespace
  $: groupedByNamespace = groupByNamespace($incidents);

  function groupByNamespace(items) {
    const groups = new Map();
    for (const inc of items) {
      const ns = inc.namespace || '(cluster-scoped)';
      if (!groups.has(ns)) groups.set(ns, []);
      groups.get(ns).push(inc);
    }
    // Sort namespaces alphabetically, but put cluster-scoped last
    const sorted = [...groups.entries()].sort((a, b) => {
      if (a[0] === '(cluster-scoped)') return 1;
      if (b[0] === '(cluster-scoped)') return -1;
      return a[0].localeCompare(b[0]);
    });
    return sorted;
  }
</script>

<div class="incident-view">
  {#if !$faultGraph}
    <div class="empty-state">
      <div class="empty-icon">K</div>
      <p>Click <strong>Scan Cluster</strong> to find configuration issues</p>
    </div>
  {:else if $incidents.length === 0}
    <div class="empty-state">
      {#if totalCount > 0}
        <p class="muted">No incidents at current severity filter ({totalCount} total)</p>
      {:else}
        <div class="all-clear">No issues found</div>
      {/if}
    </div>
  {:else}
    <div class="header">
      <div class="title-row">
        <h2>Incidents</h2>
        <span class="count">{$incidents.length}</span>
      </div>
      <div class="filters">
        <button
          class="filter-btn"
          class:active={$severityFilter === 'error'}
          on:click={() => severityFilter.set('error')}
        >
          Errors ({errorCount})
        </button>
        <button
          class="filter-btn"
          class:active={$severityFilter === 'warning'}
          on:click={() => severityFilter.set('warning')}
        >
          + Warnings ({warningCount})
        </button>
        <button
          class="filter-btn"
          class:active={$severityFilter === 'all'}
          on:click={() => severityFilter.set('all')}
        >
          All ({totalCount})
        </button>
      </div>
    </div>
    <div class="incident-list">
      {#each groupedByNamespace as [namespace, nsIncidents]}
        <div class="ns-group">
          <div class="ns-header">
            <span class="ns-name">{namespace}</span>
            <span class="ns-count">{nsIncidents.length}</span>
          </div>
          {#each nsIncidents as incident (incident.id)}
            <IncidentCard
              {incident}
              selected={false}
              on:click={() => selectIncident(incident.id)}
            />
          {/each}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .incident-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .header {
    padding: 20px 24px 12px;
    flex-shrink: 0;
  }
  .title-row {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 12px;
  }
  h2 {
    font-size: 20px;
    font-weight: 700;
  }
  .count {
    background: var(--bg-tertiary);
    color: var(--text-secondary);
    padding: 2px 10px;
    border-radius: 9999px;
    font-size: 13px;
    font-weight: 600;
  }
  .filters {
    display: flex;
    gap: 6px;
  }
  .filter-btn {
    font-size: 12px;
    padding: 4px 12px;
    border-radius: 9999px;
    background: var(--bg-primary);
    border: 1px solid var(--border);
    color: var(--text-secondary);
  }
  .filter-btn.active {
    background: var(--accent);
    border-color: var(--accent);
    color: white;
  }
  .incident-list {
    flex: 1;
    overflow-y: auto;
    padding: 8px 24px 24px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .ns-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .ns-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 0;
    position: sticky;
    top: 0;
    background: var(--bg-primary);
    z-index: 1;
  }
  .ns-name {
    font-size: 12px;
    font-weight: 600;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .ns-count {
    font-size: 11px;
    color: var(--text-muted);
    background: var(--bg-tertiary);
    padding: 1px 7px;
    border-radius: 9999px;
  }
  .empty-state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    color: var(--text-muted);
    gap: 12px;
  }
  .empty-icon {
    width: 64px;
    height: 64px;
    border-radius: 16px;
    background: var(--bg-secondary);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 32px;
    font-weight: 700;
    color: var(--accent);
  }
  .all-clear {
    font-size: 18px;
    font-weight: 600;
    color: var(--green);
  }
  .muted { font-size: 14px; }
</style>
