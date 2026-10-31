export interface NodeInfo {
  name: string;
  addr: string;
  alive: boolean;
}

export interface LinkMetrics {
  p50_ms: number;
  p95_ms: number;
  p99_ms: number;
  jitter_ms: number;
  loss_pct: number;
}

export interface TopologyData {
  nodes: NodeInfo[];
  matrix: Record<string, LinkMetrics>;
}
