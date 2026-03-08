<script>
  import { onMount, onDestroy } from 'svelte';
  import { currentContext, availableContexts, isScanning, scanError, scanTime } from '../../lib/stores/graphStore';
  import { runScan, switchContext } from '../../lib/api/wailsBridge';
  import { EventsOn } from '../../../wailsjs/runtime/runtime';
  import { GetBuildInfo } from '../../../wailsjs/go/main/App';

  let progress = null;
  let unsubProgress;
  let buildInfo = null;
  let showBuildInfo = false;

  onMount(async () => {
    unsubProgress = EventsOn('scan:progress', (data) => {
      progress = data;
    });
    try {
      buildInfo = await GetBuildInfo();
    } catch (e) {
      // ignore
    }
  });

  onDestroy(() => {
    if (unsubProgress) unsubProgress();
  });

  // Clear progress when scan finishes
  $: if (!$isScanning) {
    setTimeout(() => { progress = null; }, 2000);
  }

  function handleContextChange(e) {
    switchContext(e.target.value);
  }

  function formatTime(t) {
    if (!t) return '';
    const d = new Date(t);
    return d.toLocaleTimeString();
  }

  function validatorLabel(name) {
    const labels = {
      'reference_validation': 'References',
      'resource_limits': 'Resource Limits',
      'security': 'Security',
      'networking': 'Networking',
      'image': 'Images',
      'graph_builder': 'Building Graph',
    };
    return labels[name] || name;
  }
</script>

<header class="toolbar">
  <div class="left">
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <span class="logo"
      on:mouseenter={() => showBuildInfo = true}
      on:mouseleave={() => showBuildInfo = false}
    >
      Kogaro
      {#if showBuildInfo && buildInfo}
        <div class="build-popover">
          <div class="build-row"><span class="build-label">Commit</span> <span class="build-value">{buildInfo.commit}</span></div>
          <div class="build-row"><span class="build-label">Built</span> <span class="build-value">{buildInfo.buildTime}</span></div>
        </div>
      {/if}
    </span>
    <select value={$currentContext} on:change={handleContextChange}>
      {#each $availableContexts as ctx}
        <option value={ctx}>{ctx}</option>
      {/each}
    </select>
  </div>
  <div class="center">
    {#if progress && $isScanning}
      <div class="progress">
        <div class="progress-bar">
          <div class="progress-fill" style="width: {(progress.step / progress.totalSteps) * 100}%"></div>
        </div>
        <span class="progress-text">
          {progress.step}/{progress.totalSteps}
          {validatorLabel(progress.validator)}
          {#if progress.status === 'running'}...{:else if progress.status === 'error'}(failed){/if}
          &middot; {progress.errors} errors &middot; {progress.elapsed}
        </span>
      </div>
    {:else if $scanError}
      <span class="error-msg">{$scanError}</span>
    {:else if $scanTime}
      <span class="scan-time">Last scan: {formatTime($scanTime)}</span>
    {/if}
  </div>
  <div class="right">
    <button on:click={runScan} disabled={$isScanning}>
      {$isScanning ? 'Scanning...' : 'Scan Cluster'}
    </button>
  </div>
</header>

<style>
  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 16px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
    height: 48px;
    flex-shrink: 0;
  }
  .left, .right { display: flex; align-items: center; gap: 12px; }
  .center { flex: 1; text-align: center; display: flex; justify-content: center; }
  .logo {
    font-size: 18px;
    font-weight: 700;
    color: var(--accent);
    letter-spacing: -0.02em;
    position: relative;
    cursor: default;
  }
  .build-popover {
    position: absolute;
    top: 100%;
    left: 0;
    margin-top: 8px;
    padding: 10px 14px;
    background: var(--bg-tertiary, #27272a);
    border: 1px solid var(--border);
    border-radius: 6px;
    box-shadow: 0 4px 12px rgba(0,0,0,0.3);
    z-index: 100;
    white-space: nowrap;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .build-row {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 11px;
  }
  .build-label {
    color: var(--text-muted);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    min-width: 50px;
  }
  .build-value {
    color: var(--text-primary);
    font-family: monospace;
  }
  .error-msg { color: var(--red); font-size: 13px; }
  .scan-time { color: var(--text-muted); font-size: 12px; }
  button:disabled { opacity: 0.5; cursor: not-allowed; }

  .progress {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 300px;
  }
  .progress-bar {
    flex: 1;
    height: 6px;
    background: var(--bg-primary);
    border-radius: 3px;
    overflow: hidden;
    max-width: 160px;
  }
  .progress-fill {
    height: 100%;
    background: var(--accent);
    border-radius: 3px;
    transition: width 0.3s ease;
  }
  .progress-text {
    font-size: 12px;
    color: var(--text-secondary);
    white-space: nowrap;
  }
</style>
