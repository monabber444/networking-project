import React from 'react';
import type { NodeInfo } from '../types';

interface NodeListProps {
  nodes: NodeInfo[];
}

export const NodeList: React.FC<NodeListProps> = ({ nodes }) => {
  const sorted = [...nodes].sort((a, b) => a.name.localeCompare(b.name));
  const aliveCount = sorted.filter((n) => n.alive).length;

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-800">
        <h2 className="text-sm font-semibold text-gray-300 uppercase tracking-wider">Cluster Nodes</h2>
        <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
          {aliveCount}/{sorted.length}
        </span>
      </div>

      {/* Node list */}
      <div className="flex-1 overflow-y-auto px-2 py-2 space-y-1">
        {sorted.map((node) => (
          <div
            key={node.name}
            className="flex items-center gap-3 px-3 py-2.5 rounded-lg hover:bg-gray-800/50 transition-colors group"
          >
            {/* Status dot */}
            <span
              className={`flex-shrink-0 w-2.5 h-2.5 rounded-full ${
                node.alive
                  ? 'bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.4)]'
                  : 'bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.4)]'
              }`}
            />

            {/* Node info */}
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium text-gray-200 truncate group-hover:text-white transition-colors">
                {node.name}
              </p>
              <p className="text-xs text-gray-500 font-mono truncate">
                {node.addr}
              </p>
            </div>

            {/* Alive/Dead badge */}
            <span
              className={`flex-shrink-0 text-[10px] font-medium uppercase tracking-wider px-1.5 py-0.5 rounded ${
                node.alive
                  ? 'text-emerald-400/70'
                  : 'text-red-400/70 bg-red-500/10'
              }`}
            >
              {node.alive ? 'UP' : 'DOWN'}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
};
