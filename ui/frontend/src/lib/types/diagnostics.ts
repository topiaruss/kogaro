export interface DiagnosticFinding {
  category: string;
  summary: string;
  details: Record<string, string>;
  raw?: string;
}

export interface DiagnosticResult {
  errorCode: string;
  resourceType: string;
  resourceName: string;
  namespace: string;
  findings: DiagnosticFinding[];
  ranAt: string;
}

export interface FixCmd {
  label: string;
  command: string;
  destructive?: boolean;
}

export interface FixOption {
  label: string;
  description: string;
  risk: string;
  warnings?: string[];
  commands: FixCmd[];
  rollback?: FixCmd[];
}

export interface FixCommand {
  label: string;
  command: string;
  safe?: boolean;
  destructive?: boolean;
}

export interface PastAttemptSummary {
  treePath: string;
  result: string;
  errorMessage?: string;
  command?: string;
  when: string;
}

export interface PostCheckResult {
  stepNodeId: string;
  errorsBefore: number;
  errorsAfter: number;
  resolved?: string[];
  remaining?: string[];
  newIssues?: string[];
  status: string;
}

export interface FixStep {
  order: number;
  nodeId: string;
  resourceKind: string;
  resourceName: string;
  namespace: string;
  errorCodes: string[];
  isRootCause: boolean;
  willAutoResolve: boolean;
  dependsOn: string[];
  remediation: string;
  commands: FixCommand[];
  diagnostics: DiagnosticResult[];
  options?: FixOption[];
  warnings?: string[];
  treePath?: string;
  kbInsights?: string[];
  pastAttempts?: PastAttemptSummary[];
}

export interface FixPlan {
  incidentId: string;
  namespace: string;
  steps: FixStep[];
  generatedAt: string;
}
