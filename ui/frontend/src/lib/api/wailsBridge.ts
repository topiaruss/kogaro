import { Scan, GetNodeDetail, GetKubeContexts, GetCurrentContext, SwitchContext, GetFixPlan, RunCommand } from '../../../wailsjs/go/main/App';
import type { FixPlan } from '../types/diagnostics';
import { faultGraph, isScanning, scanError, currentContext, availableContexts, selectedNodeId } from '../stores/graphStore';
import type { NodeDetailResponse } from '../types/graph';

export async function runScan() {
  isScanning.set(true);
  scanError.set(null);
  try {
    const result = await Scan();
    faultGraph.set(result);
  } catch (err: any) {
    scanError.set(err?.message || String(err));
  } finally {
    isScanning.set(false);
  }
}

export async function fetchNodeDetail(nodeID: string): Promise<NodeDetailResponse | null> {
  try {
    return await GetNodeDetail(nodeID);
  } catch (err) {
    console.error('Failed to get node detail:', err);
    return null;
  }
}

export async function loadContexts() {
  try {
    const contexts = await GetKubeContexts();
    availableContexts.set(contexts || []);
    const current = await GetCurrentContext();
    currentContext.set(current || '');
  } catch (err) {
    console.error('Failed to load contexts:', err);
  }
}

export async function fetchFixPlan(incidentId: string): Promise<FixPlan | null> {
  try {
    return await GetFixPlan(incidentId) as FixPlan;
  } catch (err) {
    console.error('Failed to get fix plan:', err);
    return null;
  }
}

export interface FixCommandSuggestion {
  insight: string;
  nextCmds?: Array<{
    label: string;
    command: string;
    safe?: boolean;
    destructive?: boolean;
  }>;
}

export interface RunCommandResult {
  success: boolean;
  output: string;
  error?: string;
  suggestion?: FixCommandSuggestion;
}

export async function runCommand(command: string, errorCodes?: string[]): Promise<RunCommandResult> {
  try {
    return await RunCommand(command, errorCodes || []) as RunCommandResult;
  } catch (err: any) {
    return { success: false, output: '', error: err?.message || String(err) };
  }
}

export async function switchContext(name: string) {
  try {
    await SwitchContext(name);
    currentContext.set(name);
    await runScan();
  } catch (err: any) {
    scanError.set(`Failed to switch context: ${err?.message || err}`);
  }
}
