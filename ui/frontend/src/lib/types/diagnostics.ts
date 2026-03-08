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
  diagnostics: DiagnosticResult[];
}

export interface FixPlan {
  incidentId: string;
  namespace: string;
  steps: FixStep[];
  generatedAt: string;
}
