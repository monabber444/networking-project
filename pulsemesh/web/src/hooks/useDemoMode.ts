import { useEffect, useState, useRef } from 'react';
import type { TopologyData, NodeInfo, LinkMetrics } from '../types';

interface UseDemoModeReturn {
  data: TopologyData | null;
  connected: boolean;
}

const DEMO_NODES: NodeInfo[] = [
  { name: 'node-1', addr: '10.0.1.1:7946', alive: true },
  { name: 'node-2', addr: '10.0.1.2:7946', alive: true },
  { name: 'node-3', addr: '10.0.1.3:7946', alive: true },
  { name: 'node-4', addr: '10.0.1.4:7946', alive: true },
  { name: 'node-5', addr: '10.0.1.5:7946', alive: false },
];

function noise(base: number, range: number): number {
  return Math.max(0, base + (Math.random() - 0.5) * range);
}

function generateMetrics(src: string, dst: string): LinkMetrics {
  const isChaos = src === 'node-3' || dst === 'node-3';
  const isDead = src === 'node-5' || dst === 'node-5';

  if (isDead) {
    return {
      p50_ms: 0,
      p95_ms: 0,
      p99_ms: 0,
      jitter_ms: 0,
      loss_pct: 100,
    };
  }

  if (isChaos) {
    return {
      p50_ms: noise(120, 60),
      p95_ms: noise(250, 80),
      p99_ms: noise(400, 100),
      jitter_ms: noise(35, 15),
      loss_pct: noise(4.5, 3),
    };
  }

  return {
    p50_ms: noise(8, 6),
    p95_ms: noise(18, 8),
    p99_ms: noise(30, 12),
    jitter_ms: noise(2, 1.5),
    loss_pct: noise(0.1, 0.2),
  };
}

function generateTopology(): TopologyData {
  const matrix: Record<string, LinkMetrics> = {};
  for (const src of DEMO_NODES) {
    for (const dst of DEMO_NODES) {
      if (src.name === dst.name) continue;
      const key = `${src.name}->${dst.name}`;
      matrix[key] = generateMetrics(src.name, dst.name);
    }
  }
  return { nodes: [...DEMO_NODES], matrix };
}

export function useDemoMode(): UseDemoModeReturn {
  const [data, setData] = useState<TopologyData | null>(() => generateTopology());
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    intervalRef.current = setInterval(() => {
      setData(generateTopology());
    }, 1000);

    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, []);

  return { data, connected: true };
}
