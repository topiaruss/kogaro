<script>
  import { onMount } from 'svelte';
  import Toolbar from './components/layout/Toolbar.svelte';
  import SituationStrip from './components/situation/SituationStrip.svelte';
  import IncidentDetail from './components/situation/IncidentDetail.svelte';
  import GraphCanvas from './components/graph/GraphCanvas.svelte';
  import EvidencePanel from './components/evidence/EvidencePanel.svelte';
  import ImpactPanel from './components/impact/ImpactPanel.svelte';
  import { selectedIncidentId, selectedNodeId, faultGraph, visibleGraph, showCrossNamespace } from './lib/stores/graphStore';
  import { loadContexts } from './lib/api/wailsBridge';

  let viewMode = 'incidents'; // 'incidents' | 'detail' | 'graph'

  onMount(async () => {
    await loadContexts();
  });

  // When incident selection changes, switch to detail view
  $: if ($selectedIncidentId) {
    if (viewMode === 'incidents') viewMode = 'detail';
  } else {
    viewMode = 'incidents';
  }

  $: hasGraphData = $visibleGraph.nodes.length > 0;

  function goToGraph() {
    viewMode = 'graph';
  }

  function goBack() {
    if (viewMode === 'graph') {
      viewMode = 'detail';
    } else {
      selectedIncidentId.set(null);
    }
  }
</script>

<Toolbar />
<div class="main-area">
  {#if viewMode === 'graph' && hasGraphData}
    <!-- Graph drill-down mode: graph + evidence -->
    <div class="graph-layout">
      <div class="graph-area">
        <div class="graph-controls">
          <button class="back-btn" on:click={goBack}>
            &larr; Back to detail
          </button>
          <label class="ns-toggle">
            <input type="checkbox" bind:checked={$showCrossNamespace} />
            Cross-namespace
          </label>
        </div>
        <GraphCanvas />
      </div>
      {#if $selectedNodeId}
        <div class="evidence-area">
          <EvidencePanel />
        </div>
      {/if}
    </div>
  {:else if viewMode === 'detail' && $selectedIncidentId}
    <!-- Incident detail view -->
    <IncidentDetail onViewGraph={goToGraph} />
  {:else}
    <!-- Incidents-first view -->
    <SituationStrip />
  {/if}
</div>
{#if viewMode === 'graph' && hasGraphData && $faultGraph}
  <ImpactPanel />
{/if}

<style>
  .main-area {
    flex: 1;
    display: flex;
    overflow: hidden;
    min-height: 0;
  }
  .graph-layout {
    flex: 1;
    display: flex;
    overflow: hidden;
  }
  .graph-area {
    flex: 1;
    position: relative;
    min-width: 0;
  }
  .evidence-area {
    width: 360px;
    border-left: 1px solid var(--border);
    overflow-y: auto;
  }
  .graph-controls {
    position: absolute;
    top: 12px;
    left: 12px;
    z-index: 10;
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .back-btn {
    font-size: 13px;
    padding: 6px 14px;
    border-radius: 6px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    color: var(--text-primary);
  }
  .back-btn:hover {
    background: var(--bg-tertiary);
  }
  .ns-toggle {
    font-size: 12px;
    color: var(--text-secondary);
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 5px 10px;
    border-radius: 6px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    cursor: pointer;
  }
  .ns-toggle:hover {
    background: var(--bg-tertiary);
  }
  .ns-toggle input {
    accent-color: var(--accent);
  }
</style>
