import { useState, useEffect } from 'react';
import { Folder, File, ChevronRight, ChevronDown, RefreshCw, ArrowLeft } from 'lucide-react';

interface FileEntry {
  name: string; path: string; is_dir: boolean;
  size: number; mod_time: string;
}

export default function FileTree({ onSelect }: { onSelect: (entry: FileEntry) => void }) {
  const [path, setPath] = useState('.');
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = async (dir: string) => {
    setLoading(true); setError('');
    try {
      const res = await fetch(`/api/files?path=${encodeURIComponent(dir)}`);
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      setFiles(data.files || []);
      setPath(data.path || dir);
    } catch (e: any) { setError(e.message); }
    setLoading(false);
  };

  useEffect(() => { load(path); }, []);

  const navigate = (entry: FileEntry) => {
    if (entry.is_dir) { load(entry.path); }
    else { onSelect(entry); }
  };

  const goUp = () => {
    const parent = path.split('/').slice(0, -1).join('/') || '/';
    load(parent);
  };

  return (
    <div className="h-full flex flex-col bg-[#0d1117] border-r border-[#30363d] w-full">
      <div className="flex items-center justify-between px-2 py-1.5 border-b border-[#30363d] bg-[#161b22] shrink-0">
        <button onClick={goUp} className="text-[#8b949e] hover:text-[#e6edf3] p-0.5" title="Up"><ArrowLeft size={14} /></button>
        <span className="text-[10px] text-[#58a6ff] font-mono truncate mx-1">{path}</span>
        <button onClick={() => load(path)} className="text-[#8b949e] hover:text-[#e6edf3] p-0.5" title="Refresh"><RefreshCw size={12} /></button>
      </div>
      <div className="flex-1 overflow-y-auto text-xs">
        {loading ? <div className="p-2 text-[#8b949e]">Loading...</div>
        : error ? <div className="p-2 text-[#f85149]">{error}</div>
        : files.map(f => (
          <div key={f.path} onClick={() => navigate(f)}
            className={`flex items-center gap-1.5 px-2 py-1 cursor-pointer hover:bg-[#161b22] transition-colors ${
              f.is_dir ? 'text-[#58a6ff]' : 'text-[#e6edf3]'
            }`}>
            {f.is_dir ? <Folder size={14} className="shrink-0" /> : <File size={14} className="shrink-0" />}
            <span className="truncate">{f.name}</span>
            {!f.is_dir && <span className="ml-auto text-[10px] text-[#484f58] shrink-0">{formatSize(f.size)}</span>}
          </div>
        ))}
      </div>
    </div>
  );
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes}B`;
  if (bytes < 1024*1024) return `${(bytes/1024).toFixed(1)}KB`;
  return `${(bytes/(1024*1024)).toFixed(1)}MB`;
}
