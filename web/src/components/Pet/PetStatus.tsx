import { useState, useEffect } from 'react';
import { getPetState } from '../../lib/api';

export default function PetStatus() {
  const [state, setState] = useState<any>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    const poll = async () => {
      try { setState(await getPetState()); setError(''); }
      catch { setError('Cannot reach pet API.'); }
    };
    poll(); const i = setInterval(poll, 3000);
    return () => clearInterval(i);
  }, []);

  const e: Record<string,string> = { sleeping:'💤', idle:'😺', thinking:'🤔', working:'⚡', eating:'🍖' };
  const l: Record<string,string> = { sleeping:'Sleeping', idle:'Idle', thinking:'Thinking', working:'Working', eating:'Eating' };

  return (
    <div className="h-full flex items-center justify-center bg-[#0d1117]">
      <div className="text-center max-w-sm">
        {error ? <div className="text-[#f85149]"><div className="text-2xl mb-2">⚠️</div><p className="text-xs">{error}</p></div>
        : state ? <>
          <div className="text-6xl mb-4 animate-bounce">{e[state.status] || '😺'}</div>
          <p className="text-sm text-[#e6edf3] font-medium">{l[state.status] || state.status}</p>
          <div className="mt-4 space-y-1.5">
            <div className="flex justify-between text-xs"><span className="text-[#8b949e]">Agents</span><span className="text-[#58a6ff] font-mono">{state.agents}</span></div>
            <div className="flex justify-between text-xs"><span className="text-[#8b949e]">Thinking</span><span className="text-[#d2991d] font-mono">{state.thinking}</span></div>
          </div>
          <p className="text-[10px] text-[#484f58] mt-4">Desktop pet syncs in real-time</p>
        </> : <p className="text-xs text-[#8b949e]">Loading...</p>}
      </div>
    </div>
  );
}
