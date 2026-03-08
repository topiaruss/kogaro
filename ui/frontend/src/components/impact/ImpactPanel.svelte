<script>
  import { faultGraph, selectedNodeId } from '../../lib/stores/graphStore';

  let collapsed = true;

  $: dependents = getDependents($selectedNodeId, $faultGraph);

  function getDependents(nodeId, graph) {
    if (!nodeId || !graph) return [];

    // Find all nodes reachable from this node via outgoing edges
    const visited = new Set();
    const queue = [nodeId];
    while (queue.length > 0) {
      const current = queue.shift();
      if (visited.has(current)) continue;
      visited.add(current);
      for (const edge of graph.edges || []) {
        if (edge.source === current && !visited.has(edge.target)) {
          queue.push(edge.target);
        }
      }
    }

    visited.delete(nodeId);
    return graph.nodes.filter(n => visited.has(n.id));
  }

  function toggle() {
    collapsed = !collapsed;
  }
</script>

<div class="impact-panel" class:collapsed>
  <button class="toggle" on:click={toggle}>
    Impact {collapsed ? '(expand)' : '(collapse)'}
    {#if dependents.length > 0}
      <span class="count">{dependents.length} affected</span>
    {/if}
  </button>
  {#if !collapsed}
    <div class="content">
      {#if dependents.length === 0}
        <div class="empty">Select a node to see its downstream impact</div>
      {:else}
        <div class="dep-list">
          {#each dependents as dep}
            <button class="dep-item" on:click={() => selectedNodeId.set(dep.id)}>
              <span class="dep-kind">{dep.kind}</span>
              <span class="dep-name">{dep.name}</span>
              <span class="health-dot {dep.health}"></span>
            </button>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .impact-panel {
    border-top: 1px solid var(--border);
    background: var(--bg-secondary);
    flex-shrink: 0;
  }
  .toggle {
    width: 100%;
    text-align: left;
    padding: 8px 16px;
    border: none;
    border-radius: 0;
    font-size: 12px;
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .count {
    margin-left: 8px;
    color: var(--orange);
    font-weight: 400;
    text-transform: none;
  }
  .content { padding: 8px 16px 16px; }
  .collapsed .content { display: none; }
  .empty { color: var(--text-muted); font-size: 13px; }
  .dep-list { display: flex; flex-wrap: wrap; gap: 6px; }
  .dep-item {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    border-radius: 6px;
    font-size: 12px;
  }
  .dep-kind { color: var(--text-muted); font-size: 10px; text-transform: uppercase; }
  .dep-name { color: var(--text-primary); }
  .health-dot {
    width: 6px; height: 6px; border-radius: 50%;
  }
  .health-dot.broken { background: var(--red); }
  .health-dot.missing { background: var(--red); }
  .health-dot.degraded { background: var(--orange); }
  .health-dot.healthy { background: var(--green); }
  .health-dot.unknown { background: var(--text-muted); }
</style>
