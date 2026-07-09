import { useState, useRef } from 'react';
import { importBackendsFile, getBackendsExportUrl } from '../../lib/api';
import type { ImportResult } from '../../lib/types';

export default function ImportExport() {
  const [msg, setMsg] = useState('');
  const [result, setResult] = useState<ImportResult | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  const doImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setMsg('⏳ Importing...');
    try {
      const res = await importBackendsFile(file);
      setResult(res);
      setMsg(`✅ Imported ${res.count} backends`);
    } catch (err: any) {
      setMsg(`❌ ${err.message}`);
      setResult(null);
    }
  };

  return (
    <div className="overflow-auto max-h-full">
      <div className="p-5 space-y-4">
        <h2 className="text-sm font-semibold text-[#e6edf3]">📦 Import / Export</h2>

        {msg && <div className={`text-xs ${msg.startsWith('✅') ? 'text-green-400' : msg.startsWith('⏳') ? 'text-blue-400' : 'text-red-400'}`}>{msg}</div>}

        {/* Import */}
        <div className="bg-[#161b22] border border-[#30363d] rounded p-4">
          <h3 className="text-xs font-semibold text-[#e6edf3] mb-2">Import cc-switch Config</h3>
          <p className="text-[10px] text-[#8b949e] mb-3">
            Upload .yaml, .json, or .sql (SQLite dump) — backends will be merged into current config.
          </p>
          <input ref={fileRef} type="file" accept=".yaml,.yml,.json,.sql,.db"
            onChange={doImport}
            className="block w-full text-xs text-[#8b949e] file:mr-2 file:py-1 file:px-3 file:text-xs file:rounded file:border-0 file:bg-[#238636] file:text-white hover:file:bg-[#2ea043]" />
        </div>

        {result && (
          <div className="bg-[#161b22] border border-[#30363d] rounded p-3">
            <div className="text-xs text-[#e6edf3] mb-1">{result.count} backends imported</div>
            <div className="text-[10px] text-[#8b949e]">Strategy: {result.strategy}</div>
            {result.backends.slice(0, 5).map(b => (
              <div key={b.label} className="text-[10px] text-[#58a6ff] mt-1 truncate">
                {b.label} → {b.base_url}
              </div>
            ))}
          </div>
        )}

        {/* Export */}
        <div className="bg-[#161b22] border border-[#30363d] rounded p-4">
          <h3 className="text-xs font-semibold text-[#e6edf3] mb-2">Export Config</h3>
          <p className="text-[10px] text-[#8b949e] mb-3">
            Download current backends as cc-switch YAML format.
          </p>
          <a href={getBackendsExportUrl()} download="cc-switch-export.yaml"
            className="inline-block px-3 py-1 text-xs bg-[#21262d] hover:bg-[#30363d] text-[#58a6ff] rounded">
            ⬇ Download YAML
          </a>
        </div>
      </div>
    </div>
  );
}
