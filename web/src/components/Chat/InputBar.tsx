import { useState, useRef, KeyboardEvent } from 'react';
import { Send } from 'lucide-react';

interface Props { onSend: (text: string) => void; disabled?: boolean; }

export default function InputBar({ onSend, disabled }: Props) {
  const [input, setInput] = useState('');
  const taRef = useRef<HTMLTextAreaElement>(null);

  const send = () => { const t = input.trim(); if (!t || disabled) return; onSend(t); setInput(''); taRef.current?.focus(); };

  const keyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); }
    const el = taRef.current; if (el) { el.style.height = 'auto'; el.style.height = Math.min(el.scrollHeight, 150) + 'px'; }
  };

  return (
    <div className="px-3 py-2 border-t border-[#30363d] bg-[#0d1117]">
      <div className="flex items-end gap-2 bg-[#161b22] border border-[#30363d] rounded-lg px-3 py-2 focus-within:border-[#58a6ff]">
        <textarea ref={taRef} value={input}
          onChange={(e) => { setInput(e.target.value); keyDown(e as any); }}
          onKeyDown={keyDown}
          placeholder={disabled ? 'Waiting...' : 'Type a message...'}
          rows={1} disabled={disabled}
          className="flex-1 bg-transparent text-[#e6edf3] text-sm resize-none outline-none placeholder-[#484f58] max-h-[150px] py-0.5" />
        <button onClick={send} disabled={disabled || !input.trim()}
          className={`p-1.5 rounded-md ${disabled || !input.trim() ? 'text-[#484f58]' : 'text-[#58a6ff] hover:bg-[#58a6ff]/20'}`}>
          <Send size={16} />
        </button>
      </div>
    </div>
  );
}
