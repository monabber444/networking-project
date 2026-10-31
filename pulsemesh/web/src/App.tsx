import { useState, useCallback, useEffect } from 'react';
import { Header } from './components/Header';
import { NodeList } from './components/NodeList';
import { MatrixGrid } from './components/MatrixGrid';
import { MetricCard } from './components/MetricCard';
import { useWebSocket } from './hooks/useWebSocket';
import { useDemoMode } from './hooks/useDemoMode';
import type { TopologyData } from './types';

function App() {
  const ws = useWebSocket();
  const demo = useDemoMode();

  // If WebSocket is connected, prefer live data unless user explicitly toggled demoMode
  const [demoMode, setDemoMode] = useState(false);
  const [selectedLink, setSelectedLink] = useState<{ src: string; dst: string } | null>(null);
  const [history, setHistory] = useState<Record<string, number[]>>({});

  const source: { data: TopologyData | null; connected: boolean } = demoMode ? demo : ws;
  const topology = source.data;

  // Track historical latency for sparklines
  useEffect(() => {
    if (!topology?.matrix) return;
    setHistory((prev) => {
      const next = { ...prev };
      for (const [key, m] of Object.entries(topology.matrix)) {
        const arr = next[key] ? [...next[key]] : [];
        arr.push(m.p50_ms);
        if (arr.length > 30) arr.shift();
        next[key] = arr;
      }
      return next;
    });
  }, [topology]);

  const handleSelectLink = useCallback((src: string, dst: string) => {
    setSelectedLink((prev) =>
      prev?.src === src && prev?.dst === dst ? null : { src, dst }
    );
  }, []);

  const selectedMetrics = selectedLink && topology
    ? topology.matrix[`${selectedLink.src}->${selectedLink.dst}`]
    : undefined;

  const reverseMetrics = selectedLink && topology
    ? topology.matrix[`${selectedLink.dst}->${selectedLink.src}`]
    : undefined;

  return (
    <div className="flex flex-col h-screen bg-gray-950 text-white antialiased">
      <Header
        connected={source.connected}
        demoMode={demoMode}
        onToggleDemo={() => setDemoMode((v) => !v)}
      />

      <div className="flex-1 flex flex-col lg:flex-row overflow-hidden">
        {/* Left sidebar: Node list */}
        <aside className="w-full lg:w-60 xl:w-64 flex-shrink-0 border-r border-gray-800 bg-gray-900/40 overflow-y-auto lg:block">
          {topology ? (
            <NodeList nodes={topology.nodes} />
          ) : (
            <div className="flex items-center justify-center h-full text-gray-600 text-sm p-8">
              Waiting for data…
            </div>
          )}
        </aside>

        {/* Center: Matrix heatmap */}
        <main className="flex-1 min-w-0 overflow-auto bg-gray-950">
          {topology ? (
            <MatrixGrid
              topology={topology}
              selectedLink={selectedLink}
              onSelectLink={handleSelectLink}
            />
          ) : (
            <div className="flex items-center justify-center h-full">
              <div className="text-center text-gray-600">
                <div className="relative w-16 h-16 mx-auto mb-4">
                  <div className="absolute inset-0 rounded-full border-2 border-gray-800 border-t-emerald-500 animate-spin" />
                </div>
                <p className="text-sm">Connecting to PulseMesh…</p>
              </div>
            </div>
          )}
        </main>

        {/* Right sidebar: Metric details */}
        <aside
          className={`
            w-full lg:w-72 xl:w-80 flex-shrink-0 border-l border-gray-800 bg-gray-900/40
            transition-all duration-300 overflow-y-auto
            ${selectedLink && selectedMetrics ? 'lg:block' : 'hidden'}
          `}
        >
          {selectedLink && selectedMetrics && (
            <MetricCard
              source={selectedLink.src}
              target={selectedLink.dst}
              metrics={selectedMetrics}
              reverseMetrics={reverseMetrics}
              history={history[`${selectedLink.src}->${selectedLink.dst}`]}
            />
          )}
        </aside>

        {/* No selection placeholder (when no link is selected on large screens) */}
        {!selectedLink && (
          <aside className="hidden lg:flex w-72 xl:w-80 flex-shrink-0 border-l border-gray-800 bg-gray-900/40 items-center justify-center">
            <div className="text-center text-gray-700 px-6">
              <svg className="w-10 h-10 mx-auto mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M15 15l-2 5L9 9l11 4-5 2zm0 0l5 5M7.188 2.239l.777 2.897M5.136 7.965l-2.898-.777M13.95 4.05l-2.122 2.122m-5.657 5.656l-2.12 2.122" />
              </svg>
              <p className="text-sm">Click a cell to inspect link metrics</p>
            </div>
          </aside>
        )}
      </div>
    </div>
  );
}

export default App;
