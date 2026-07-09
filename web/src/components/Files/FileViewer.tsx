import { useState, useEffect } from 'react';
import FileTree from './FileTree';

interface FileEntry {
  name: string; path: string; is_dir: boolean; size: number;
}

export default function FileViewer() {
  const [file, setFile] = useState<FileEntry | null>(null);
  const [content, setContent] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!file || file.is_dir) return;
    setLoading(true); setError('');
    fetch(`/api/files/content?path=${encodeURIComponent(file.path)}`)
      .then(async r => {
        if (!r.ok) throw new Error(await r.text());
        const d = await r.json();
        if (d.binary) { setContent('[Binary file]'); return; }
        setContent(d.content || '');
      })
      .catch((e: any) => setError(e.message))
      .finally(() => setLoading(false));
  }, [file]);

  return (
    <div className="h-full flex">
      <div className="w-56 shrink-0"><FileTree onSelect={setFile} /></div>
      <div className="flex-1 flex flex-col overflow-hidden bg-[#0d1117]">
        <div className="flex items-center px-3 py-1.5 border-b border-[#30363d] bg-[#161b22] shrink-0">
          <span className="text-[10px] text-[#8b949e] font-mono truncate">
            {file ? file.path : 'Select a file'}
          </span>
          {file && !file.is_dir && (
            <span className="ml-auto text-[10px] text-[#484f58]">
              {file.name} · {content ? content.split('\n').length : 0} lines
            </span>
          )}
        </div>
        <div className="flex-1 overflow-auto">
          {loading ? <div className="p-3 text-xs text-[#8b949e]">Loading...</div>
          : error ? <div className="p-3 text-xs text-[#f85149]">{error}</div>
          : content ? (
            <pre className="p-3 text-xs font-mono text-[#e6edf3] leading-relaxed whitespace-pre">
              {content.length > 50000
                ? content.slice(0, 50000) + `\n\n... (${content.length - 50000} more chars)`
                : content}
            </pre>
          ) : file ? (
            <div className="p-3 text-xs text-[#8b949e]">Empty file</div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
