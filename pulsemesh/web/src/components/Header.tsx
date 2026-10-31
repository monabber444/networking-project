import React from 'react';

interface HeaderProps {
  connected: boolean;
  demoMode: boolean;
  onToggleDemo: () => void;
}

export const Header: React.FC<HeaderProps> = ({ connected, demoMode, onToggleDemo }) => {
  const statusColor = demoMode
    ? 'bg-blue-400'
    : connected
      ? 'bg-emerald-400'
      : 'bg-red-500';

  const statusLabel = demoMode
    ? 'Demo'
    : connected
      ? 'Connected'
      : 'Disconnected';

  return (
    <header className="flex items-center justify-between px-6 py-3 bg-gray-900/80 border-b border-gray-800 backdrop-blur-md sticky top-0 z-50">
      {/* Logo */}
      <div className="flex items-center gap-3">
        <div className="relative flex items-center justify-center w-9 h-9">
          <span className="absolute inset-0 rounded-lg bg-emerald-500/20 animate-ping" />
          <span className="relative flex items-center justify-center w-9 h-9 rounded-lg bg-emerald-500/10 border border-emerald-500/30">
            <svg className="w-5 h-5 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
          </span>
        </div>
        <div>
          <h1 className="text-lg font-bold tracking-tight text-white">
            Pulse<span className="text-emerald-400">Mesh</span>
          </h1>
          <p className="text-[10px] uppercase tracking-widest text-gray-500 -mt-0.5">Network Health Matrix</p>
        </div>
      </div>

      {/* Right controls */}
      <div className="flex items-center gap-5">
        {/* Connection status */}
        <div className="flex items-center gap-2 text-sm text-gray-400">
          <span className={`inline-block w-2 h-2 rounded-full ${statusColor} shadow-[0_0_6px] ${demoMode ? 'shadow-blue-400/50' : connected ? 'shadow-emerald-400/50' : 'shadow-red-500/50'}`} />
          <span>{statusLabel}</span>
        </div>

        {/* Demo mode toggle */}
        <label className="flex items-center gap-2 cursor-pointer select-none group">
          <span className="text-xs text-gray-500 group-hover:text-gray-300 transition-colors">Demo</span>
          <div className="relative">
            <input
              type="checkbox"
              checked={demoMode}
              onChange={onToggleDemo}
              className="sr-only peer"
            />
            <div className="w-9 h-5 rounded-full bg-gray-700 peer-checked:bg-emerald-600 transition-colors duration-200" />
            <div className="absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow-md transform peer-checked:translate-x-4 transition-transform duration-200" />
          </div>
        </label>
      </div>
    </header>
  );
};
