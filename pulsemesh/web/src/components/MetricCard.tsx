import React from 'react';
import type { LinkMetrics } from '../types';

interface MetricCardProps {
  source: string;
  target: string;
  metrics: LinkMetrics;
  reverseMetrics?: LinkMetrics;
  history?: number[];
}

const Sparkline: React.FC<{ data: number[] }> = ({ data }) => {
  if (!data || data.length < 2) {
    return (
      <div className="h-14 flex items-center justify-center text-[11px] text-gray-500 bg-gray-900/40 rounded-lg border border-gray-800">
        Collecting latency history…
      </div>
    );
  }

  const width = 240;
  const height = 48;
  const min = Math.min(...data);
  const max = Math.max(...data);
  const range = max - min || 1;

  const points = data
    .map((val, idx) => {
      const x = (idx / (data.length - 1)) * width;
      const y = height - ((val - min) / range) * (height - 14) - 7;
      return `${x},${y}`;
    })
    .join(' ');

  const areaPoints = `${points} ${width},${height} 0,${height}`;
  const latest = data[data.length - 1];

  return (
    <div className="p-3 rounded-lg bg-gray-900/60 border border-gray-800 space-y-1.5">
      <div className="flex items-center justify-between text-[11px]">
        <span className="text-gray-400 font-medium uppercase tracking-wider">RTT Sparkline (p50)</span>
        <span className="font-mono text-emerald-400 font-semibold">{latest.toFixed(2)}ms</span>
      </div>
      <div className="relative">
        <svg viewBox={`0 0 ${width} ${height}`} className="w-full h-12 overflow-visible">
          <defs>
            <linearGradient id="sparklineGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#10b981" stopOpacity="0.4" />
              <stop offset="100%" stopColor="#10b981" stopOpacity="0.0" />
            </linearGradient>
          </defs>
          <polygon points={areaPoints} fill="url(#sparklineGrad)" />
          <polyline
            fill="none"
            stroke="#10b981"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            points={points}
          />
        </svg>
      </div>
      <div className="flex justify-between text-[10px] text-gray-500 font-mono">
        <span>min: {min.toFixed(1)}ms</span>
        <span>max: {max.toFixed(1)}ms</span>
      </div>
    </div>
  );
};

function latencyColor(ms: number): string {
  if (ms < 20) return 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20';
  if (ms < 100) return 'text-amber-400 bg-amber-500/10 border-amber-500/20';
  return 'text-red-400 bg-red-500/10 border-red-500/20';
}

function lossColor(pct: number): string {
  if (pct < 1) return 'text-emerald-400';
  if (pct <= 5) return 'text-amber-400';
  return 'text-red-400';
}

function lossBarColor(pct: number): string {
  if (pct < 1) return 'bg-emerald-500';
  if (pct <= 5) return 'bg-amber-500';
  return 'bg-red-500';
}

const MetricBadge: React.FC<{ label: string; value: string; colorClass: string }> = ({ label, value, colorClass }) => (
  <div className={`flex flex-col items-center px-3 py-2 rounded-lg border ${colorClass}`}>
    <span className="text-[10px] uppercase tracking-wider text-gray-500 mb-1">{label}</span>
    <span className="text-sm font-bold font-mono">{value}</span>
  </div>
);

const DirectionMetrics: React.FC<{ src: string; dst: string; m: LinkMetrics }> = ({ src, dst, m }) => (
  <div className="space-y-3">
    {/* Direction header */}
    <div className="flex items-center gap-2 text-sm">
      <span className="font-medium text-gray-200">{src}</span>
      <svg className="w-4 h-4 text-emerald-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M13 7l5 5m0 0l-5 5m5-5H6" />
      </svg>
      <span className="font-medium text-gray-200">{dst}</span>
    </div>

    {/* Latency badges */}
    <div className="grid grid-cols-3 gap-2">
      <MetricBadge label="p50" value={`${m.p50_ms.toFixed(1)}ms`} colorClass={latencyColor(m.p50_ms)} />
      <MetricBadge label="p95" value={`${m.p95_ms.toFixed(1)}ms`} colorClass={latencyColor(m.p95_ms)} />
      <MetricBadge label="p99" value={`${m.p99_ms.toFixed(1)}ms`} colorClass={latencyColor(m.p99_ms)} />
    </div>

    {/* Jitter */}
    <div className="flex items-center justify-between px-3 py-2 rounded-lg bg-gray-800/50 border border-gray-700/50">
      <span className="text-xs text-gray-500 uppercase tracking-wider">Jitter</span>
      <span className="text-sm font-mono font-medium text-gray-300">
        ±{m.jitter_ms.toFixed(1)}ms
      </span>
    </div>

    {/* Packet loss */}
    <div className="px-3 py-2 rounded-lg bg-gray-800/50 border border-gray-700/50">
      <div className="flex items-center justify-between mb-1.5">
        <span className="text-xs text-gray-500 uppercase tracking-wider">Packet Loss</span>
        <span className={`text-sm font-mono font-bold ${lossColor(m.loss_pct)}`}>
          {m.loss_pct.toFixed(2)}%
        </span>
      </div>
      <div className="w-full h-1.5 bg-gray-700 rounded-full overflow-hidden">
        <div
          className={`h-full rounded-full transition-all duration-500 ${lossBarColor(m.loss_pct)}`}
          style={{ width: `${Math.min(m.loss_pct, 100)}%` }}
        />
      </div>
    </div>
  </div>
);

export const MetricCard: React.FC<MetricCardProps> = ({ source, target, metrics, reverseMetrics, history }) => {
  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="px-4 py-3 border-b border-gray-800">
        <h2 className="text-sm font-semibold text-gray-300 uppercase tracking-wider">Link Details</h2>
      </div>

      <div className="flex-1 overflow-y-auto px-4 py-4 space-y-5">
        {/* Real-time Sparkline */}
        <Sparkline data={history || []} />

        {/* Forward direction */}
        <DirectionMetrics src={source} dst={target} m={metrics} />

        {/* Reverse direction */}
        {reverseMetrics && (
          <>
            <div className="border-t border-gray-800" />
            <DirectionMetrics src={target} dst={source} m={reverseMetrics} />
          </>
        )}

        {/* Asymmetry indicator */}
        {reverseMetrics && (
          <div className="px-3 py-2 rounded-lg bg-indigo-500/5 border border-indigo-500/20">
            <div className="flex items-center gap-2 mb-1">
              <svg className="w-3.5 h-3.5 text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4" />
              </svg>
              <span className="text-xs text-indigo-400 font-medium uppercase tracking-wider">Asymmetry</span>
            </div>
            <p className="text-xs text-gray-500">
              Δ p50: <span className="font-mono text-gray-400">
                {Math.abs(metrics.p50_ms - reverseMetrics.p50_ms).toFixed(1)}ms
              </span>
              {' · '}
              Δ loss: <span className="font-mono text-gray-400">
                {Math.abs(metrics.loss_pct - reverseMetrics.loss_pct).toFixed(2)}%
              </span>
            </p>
          </div>
        )}
      </div>
    </div>
  );
};
