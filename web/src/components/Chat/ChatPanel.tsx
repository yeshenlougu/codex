import { useState, useRef, useCallback } from 'react';
import type { Message, WSMessage } from '../../lib/types';
import { streamMessage } from '../../lib/api';
import { useWebSocket } from '../../hooks/useWebSocket';
import MessageBubble from './MessageBubble';
import InputBar from './InputBar';

export default function ChatPanel({ sessionId }: { sessionId: string }) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [streaming, setStreaming] = useState(false);
  const [streamingText, setStreamingText] = useState('');
  const [error, setError] = useState<string | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  const scroll = () => setTimeout(() => bottomRef.current?.scrollIntoView({ behavior: 'smooth' }), 50);

  useWebSocket({ sessionId, enabled: true,
    onMessage: (msg: WSMessage) => {
      if (msg.type === 'chunk' && msg.content) setStreamingText((p) => p + msg.content);
      else if (msg.type === 'done') { setStreaming(false); setStreamingText(''); if (msg.content) setMessages((p) => [...p, { role: 'assistant', content: msg.content! }]); scroll(); }
      else if (msg.type === 'error') { setError(msg.error || 'Unknown'); setStreaming(false); }
    }
  });

  const send = useCallback(async (text: string) => {
    setError(null); setMessages((p) => [...p, { role: 'user', content: text }]); setStreaming(true); setStreamingText(''); scroll();
    try {
      await streamMessage(text, sessionId,
        (c) => setStreamingText((p) => p + c),
        (full) => { setMessages((p) => [...p, { role: 'assistant', content: full }]); setStreaming(false); setStreamingText(''); scroll(); },
        (e) => { setError(e); setStreaming(false); }
      );
    } catch (e: any) { setError(e.message); setStreaming(false); }
  }, [sessionId]);

  return (
    <div className="flex flex-col h-full">
      <div className="flex-1 overflow-y-auto px-4 py-3 space-y-3">
        {messages.length === 0 && !streaming && (
          <div className="flex items-center justify-center h-full">
            <div className="text-center text-[#8b949e]">
              <div className="text-4xl mb-3">💬</div>
              <p className="text-sm font-medium">Codex Go</p>
              <p className="text-xs mt-1">Ask me anything about your code</p>
              <div className="mt-4 flex gap-2 justify-center text-xs">
                {['shell','read_file','grep','git','write_file','ls','web_fetch','edit_file'].map(t => (
                  <span key={t} className="px-2 py-0.5 bg-[#21262d] rounded">{t}</span>
                ))}
              </div>
            </div>
          </div>
        )}
        {messages.map((m, i) => <MessageBubble key={i} message={m} />)}
        {streaming && (
          <div className="message-bubble p-3 bg-[#161b22] border border-[#30363d] rounded-lg text-sm text-[#e6edf3]">
            {streamingText}<span className="streaming-cursor" />
          </div>
        )}
        {error && <div className="p-2 bg-red-900/20 border border-red-900/50 rounded text-xs text-[#f85149]">Error: {error}
          <button onClick={() => setError(null)} className="ml-2 underline">Dismiss</button></div>}
        <div ref={bottomRef} />
      </div>
      <InputBar onSend={send} disabled={streaming} />
    </div>
  );
}
