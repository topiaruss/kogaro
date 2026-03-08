<script>
  import { onMount, onDestroy } from 'svelte';
  import { visibleGraph, selectedNodeId, selectedIncidentId, faultGraph, incidents } from '../../lib/stores/graphStore';

  let container;
  let cy = null;

  function getNodeColor(node, isPrimary, isConnected) {
    if (isPrimary) {
      if (node.health === 'broken' || node.health === 'missing') return '#ef4444';
      if (node.health === 'degraded') return '#f97316';
      return '#a1a1aa';
    }
    if (isConnected) {
      // Muted versions — context, not alarm
      if (node.health === 'broken' || node.health === 'missing') return '#7f1d1d';
      if (node.health === 'degraded') return '#7c2d12';
      return '#52525b';
    }
    return '#3f3f46';
  }

  function getNodeBorder(node, isPrimary, isConnected) {
    if (isPrimary) {
      if (node.isFaultOrigin) return { width: 3, color: '#ef4444' };
      if (node.isRootCause) return { width: 3, color: '#ef4444', style: 'dashed' };
      if (node.health === 'degraded') return { width: 2, color: '#f97316' };
      return { width: 2, color: '#71717a' };
    }
    if (isConnected) {
      if (node.health === 'broken' || node.health === 'missing') return { width: 1, color: '#991b1b' };
      return { width: 1, color: '#52525b' };
    }
    return { width: 1, color: '#3f3f46' };
  }

  function getNodeShape(kind) {
    const shapes = {
      'Pod': 'ellipse',
      'Deployment': 'round-rectangle',
      'ReplicaSet': 'round-rectangle',
      'StatefulSet': 'round-rectangle',
      'Service': 'diamond',
      'Ingress': 'pentagon',
      'ConfigMap': 'hexagon',
      'Secret': 'hexagon',
      'PersistentVolumeClaim': 'rectangle',
      'ServiceAccount': 'octagon',
      'IngressClass': 'triangle',
      'StorageClass': 'triangle',
      'NetworkPolicy': 'star',
    };
    return shapes[kind] || 'ellipse';
  }

  function nodeLabel(node) {
    let label = node.name;
    if (label.length > 24) label = label.slice(0, 22) + '..';
    let text = `${node.kind}\n${label}`;
    if (node.resourceCount > 1) text += `\n(x${node.resourceCount})`;
    return text;
  }

  // Determine which nodes are "primary" — directly part of the selected incident
  function getPrimaryNodeIds() {
    const incId = currentIncidentId;
    if (!incId || !currentFaultGraph) return new Set();
    const inc = currentFaultGraph.incidents?.find(i => i.id === incId);
    if (!inc) return new Set();
    const ids = new Set(inc.affectedNodes || []);
    (inc.rootCauses || []).forEach(rc => ids.add(rc));
    return ids;
  }

  let currentIncidentId = null;
  let currentFaultGraph = null;

  const unsubIncident = selectedIncidentId.subscribe(v => { currentIncidentId = v; });
  const unsubFG = faultGraph.subscribe(v => { currentFaultGraph = v; });

  function buildCyElements(graph) {
    const primaryIds = getPrimaryNodeIds();

    // Also find edges connected to primary nodes to mark them
    const primaryEdgeNodes = new Set(primaryIds);
    (graph.edges || []).forEach(e => {
      if (primaryIds.has(e.source)) primaryEdgeNodes.add(e.target);
      if (primaryIds.has(e.target)) primaryEdgeNodes.add(e.source);
    });

    const nodes = (graph.nodes || []).map(n => {
      const isPrimary = primaryIds.has(n.id);
      const isConnected = !isPrimary && primaryEdgeNodes.has(n.id);
      const opacity = isPrimary ? 1.0 : isConnected ? 0.4 : 0.12;
      const nodeSize = isPrimary ? 75 : isConnected ? 45 : 35;
      const border = getNodeBorder(n, isPrimary, isConnected);

      return {
        data: {
          id: n.id,
          label: nodeLabel(n),
          kind: n.kind,
          health: n.health,
          borderWidth: border.width,
          borderColor: border.color,
          borderStyle: border.style || 'solid',
          bgColor: getNodeColor(n, isPrimary, isConnected),
          opacity: opacity,
          shape: getNodeShape(n.kind),
          nodeSize: nodeSize,
          nodeData: n,
        }
      };
    });

    const edges = (graph.edges || []).map((e, i) => {
      const touchesPrimary = primaryIds.has(e.source) || primaryIds.has(e.target);
      return {
        data: {
          id: `e-${i}`,
          source: e.source,
          target: e.target,
          label: e.label || '',
          lineColor: e.health === 'broken' ? '#ef4444' : touchesPrimary ? '#71717a' : '#2a2a2e',
          lineWidth: e.health === 'broken' ? 3 : touchesPrimary ? 2 : 1,
          lineStyle: e.health === 'broken' ? 'dashed' : 'solid',
          opacity: touchesPrimary ? 1.0 : 0.2,
        }
      };
    });

    return [...nodes, ...edges];
  }

  async function initCytoscape(elements) {
    const cytoscape = (await import('cytoscape')).default;
    const dagre = (await import('cytoscape-dagre')).default;
    cytoscape.use(dagre);

    if (cy) cy.destroy();

    cy = cytoscape({
      container,
      elements,
      style: [
        {
          selector: 'node',
          style: {
            'label': 'data(label)',
            'text-wrap': 'wrap',
            'text-valign': 'center',
            'text-halign': 'center',
            'font-size': '10px',
            'color': '#fafafa',
            'text-outline-width': 2,
            'text-outline-color': '#18181b',
            'background-color': 'data(bgColor)',
            'border-width': 'data(borderWidth)',
            'border-color': 'data(borderColor)',
            'border-style': 'data(borderStyle)',
            'opacity': 'data(opacity)',
            'shape': 'data(shape)',
            'width': 'data(nodeSize)',
            'height': 'data(nodeSize)',
          }
        },
        {
          selector: 'edge',
          style: {
            'label': 'data(label)',
            'font-size': '9px',
            'color': '#a1a1aa',
            'text-outline-width': 1,
            'text-outline-color': '#18181b',
            'line-color': 'data(lineColor)',
            'width': 'data(lineWidth)',
            'line-style': 'data(lineStyle)',
            'line-opacity': 'data(opacity)',
            'target-arrow-color': 'data(lineColor)',
            'target-arrow-shape': 'triangle',
            'curve-style': 'bezier',
            'arrow-scale': 0.8,
          }
        },
        {
          selector: 'node:selected',
          style: {
            'border-color': '#3b82f6',
            'border-width': 3,
            'overlay-opacity': 0.1,
            'overlay-color': '#3b82f6',
          }
        }
      ],
      layout: {
        name: 'dagre',
        rankDir: 'TB',
        spacingFactor: 1.5,
        nodeDimensionsIncludeLabels: true,
      },
      minZoom: 0.3,
      maxZoom: 3,
    });

    cy.on('tap', 'node', (evt) => {
      const nodeData = evt.target.data('nodeData');
      if (nodeData) selectedNodeId.set(nodeData.id);
    });

    cy.on('tap', (evt) => {
      if (evt.target === cy) selectedNodeId.set(null);
    });

    // Auto-fit to show primary nodes prominently
    cy.fit(undefined, 40);
  }

  const unsubscribe = visibleGraph.subscribe(graph => {
    if (!container) return;
    if (!graph.nodes || graph.nodes.length === 0) {
      if (cy) cy.destroy();
      cy = null;
      return;
    }
    const elements = buildCyElements(graph);
    initCytoscape(elements);
  });

  onMount(() => {
    const currentGraph = { nodes: [], edges: [] };
    visibleGraph.subscribe(g => Object.assign(currentGraph, g))();
    if (currentGraph.nodes.length > 0) {
      initCytoscape(buildCyElements(currentGraph));
    }
  });

  onDestroy(() => {
    unsubscribe();
    unsubIncident();
    unsubFG();
    if (cy) cy.destroy();
  });
</script>

<div class="canvas-container" bind:this={container}>
  {#if !$faultGraph || $visibleGraph.nodes.length === 0}
    <div class="empty-state">
      <p>Select a node to see details</p>
    </div>
  {/if}
</div>

<style>
  .canvas-container {
    width: 100%;
    height: 100%;
    position: relative;
  }
  .empty-state {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    color: var(--text-muted);
    gap: 12px;
  }
  .empty-state p { font-size: 14px; }
</style>
