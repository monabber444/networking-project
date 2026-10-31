import React, { useMemo } from 'react';
import type { TopologyData, LinkMetrics } from '../types';

interface MatrixGridProps {
  topology: TopologyData;
  selectedLink: { src: string; dst: string } | null;
  onSelectLink: (src: string, dst: string) => void;
}

/**
 * Compute cell background color based on p50 latency and loss.
 * Uses smooth interpolation within ranges.
 */
function cellStyle(m: LinkMetrics | undefined): { bg: string; glow: string; text: string } {
  if (!m) return { bg: 'bg-gray-800', glow: '', text: 'text-gray-600' };

  const { p50_ms, loss_pct } = m;

  // Dead link
  if (loss_pct >= 100) {
    return { bg: 'bg-gray-800/80', glow: '', text: 'text-gray-600' };
  }

  // Crimson: over 100ms OR over 2% loss
  if (p50_ms > 100 || loss_pct > 2) {
    const intensity = Math.min(1, Math.max(0.4, (p50_ms - 100) / 200 + loss_pct / 10));
    return {
      bg: `bg-red-500`,
      glow: `shadow-[0_0_${Math.round(intensity * 12)}px_rgba(239,68,68,${(intensity * 0.5).toFixed(2)})]`,
      text: 'text-white',
    };
  }

  // Amber: 20-100ms OR under 2% loss (but some loss)
  if (p50_ms >= 20 || loss_pct >= 0.5) {
    return {
      bg: 'bg-amber-500',
      glow: 'shadow-[0_0_8px_rgba(245,158,11,0.3)]',
      text: 'text-gray-900',
    };
  }

  // Emerald: under 20ms AND 0% loss (effectively)
  return {
    bg: 'bg-emerald-500',
    glow: 'shadow-[0_0_8px_rgba(16,185,129,0.3)]',
    text: 'text-gray-900',
  };
}

function getCellOpacity(m: LinkMetrics | undefined): number {
  if (!m || m.loss_pct >= 100) return 0.3;
  // Vary opacity based on p50 for depth effect
  if (m.p50_ms < 10) return 0.7;
  if (m.p50_ms < 50) return 0.85;
  return 1;
}

export const MatrixGrid: React.FC<MatrixGridProps> = ({ topology, selectedLink, onSelectLink }) => {
  const nodeNames = useMemo(
    () => topology.nodes.map((n) => n.name).sort(),
    [topology.nodes]
  );

  const n = nodeNames.length;

  if (n === 0) {
    return (
      <div className="flex items-center justify-center h-full text-gray-500">
        <div className="text-center">
          <svg className="w-12 h-12 mx-auto mb-3 text-gray-700" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6A2.25 2.25 0 016 3.75h2.25A2.25 2.25 0 0110.5 6v2.25a2.25 2.25 0 01-2.25 2.25H6a2.25 2.25 0 01-2.25-2.25V6zM3.75 15.75A2.25 2.25 0 016 13.5h2.25a2.25 2.25 0 012.25 2.25V18a2.25 2.25 0 01-2.25 2.25H6A2.25 2.25 0 013.75 18v-2.25zM13.5 6a2.25 2.25 0 012.25-2.25H18A2.25 2.25 0 0120.25 6v2.25A2.25 2.25 0 0118 10.5h-2.25a2.25 2.25 0 01-2.25-2.25V6zM13.5 15.75a2.25 2.25 0 012.25-2.25H18a2.25 2.25 0 012.25 2.25V18A2.25 2.25 0 0118 20.25h-2.25A2.25 2.25 0 0113.5 18v-2.25z" />
          </svg>
          <p className="text-sm">Waiting for node data…</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full px-4 py-4">
      {/* Title */}
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-sm font-semibold text-gray-300 uppercase tracking-wider">
          Latency Heatmap
        </h2>
        <div className="flex items-center gap-3 text-[10px] text-gray-500 uppercase tracking-wider">
          <span className="flex items-center gap-1.5"><span className="w-3 h-3 rounded-sm bg-emerald-500 inline-block" /> &lt;20ms</span>
          <span className="flex items-center gap-1.5"><span className="w-3 h-3 rounded-sm bg-amber-500 inline-block" /> 20–100ms</span>
          <span className="flex items-center gap-1.5"><span className="w-3 h-3 rounded-sm bg-red-500 inline-block" /> &gt;100ms</span>
        </div>
      </div>

      {/* Grid container */}
      <div className="flex-1 overflow-auto">
        <div
          className="inline-grid gap-[2px]"
          style={{
            gridTemplateColumns: `minmax(80px, auto) repeat(${n}, minmax(64px, 1fr))`,
            gridTemplateRows: `auto repeat(${n}, minmax(48px, 1fr))`,
          }}
        >
          {/* Empty top-left corner */}
          <div className="flex items-end justify-center pb-2 pr-2">
            <span className="text-[10px] text-gray-600 uppercase tracking-wider">src ↓ / dst →</span>
          </div>

          {/* Column headers (targets) */}
          {nodeNames.map((name) => (
            <div
              key={`col-${name}`}
              className="flex items-end justify-center pb-2 px-1"
            >
              <span className="text-xs font-medium text-gray-400 truncate max-w-[80px] -rotate-0" title={name}>
                {name}
              </span>
            </div>
          ))}

          {/* Grid rows */}
          {nodeNames.map((src) => (
            <React.Fragment key={`row-${src}`}>
              {/* Row header (source) */}
              <div className="flex items-center justify-end pr-3">
                <span className="text-xs font-medium text-gray-400 truncate max-w-[80px]" title={src}>
                  {src}
                </span>
              </div>

              {/* Cells */}
              {nodeNames.map((dst) => {
                const isSelf = src === dst;
                const key = `${src}->${dst}`;
                const m = topology.matrix[key];
                const isSelected = selectedLink?.src === src && selectedLink?.dst === dst;

                if (isSelf) {
                  return (
                    <div
                      key={key}
                      className="flex items-center justify-center rounded-md bg-gray-800/40 border border-gray-800/60"
                    >
                      <span className="text-[10px] text-gray-700">—</span>
                    </div>
                  );
                }

                const style = cellStyle(m);
                const opacity = getCellOpacity(m);

                return (
                  <button
                    key={key}
                    onClick={() => onSelectLink(src, dst)}
                    className={`
                      relative flex items-center justify-center rounded-md cursor-pointer
                      transition-all duration-300 ease-in-out
                      border-2
                      ${style.bg} ${style.glow}
                      ${isSelected
                        ? 'border-white ring-2 ring-white/20 scale-105 z-10'
                        : 'border-transparent hover:border-white/30 hover:scale-[1.03]'
                      }
                    `}
                    style={{ opacity }}
                    title={m ? `${src} → ${dst}: p50=${m.p50_ms.toFixed(1)}ms loss=${m.loss_pct.toFixed(1)}%` : `${src} → ${dst}: no data`}
                  >
                    <span className={`text-xs font-mono font-bold ${style.text} transition-all duration-300`}>
                      {m ? (m.loss_pct >= 100 ? '✕' : `${m.p50_ms.toFixed(0)}`) : '?'}
                    </span>
                    {m && m.loss_pct > 0 && m.loss_pct < 100 && (
                      <span className="absolute bottom-0.5 right-1 text-[8px] font-mono opacity-70">
                        {m.loss_pct.toFixed(0)}%
                      </span>
                    )}
                  </button>
                );
              })}
            </React.Fragment>
          ))}
        </div>
      </div>
    </div>
  );
};
