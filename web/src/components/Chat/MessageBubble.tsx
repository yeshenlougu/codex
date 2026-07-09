import type { Message } from '../../lib/types';

export default function MessageBubble({ message }: { message: Message }) {
  const { role, content, tool_calls } = message;
  if (role === 'system') return null;
  const isUser = role === 'user';
  const isTool = role === 'tool';

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'}`}>
      <div className={`max-w-[80%] rounded-lg px-4 py-2.5 text-sm ${
        isUser ? 'bg-[#238636] text-white rounded-br-sm' :
        isTool ? 'bg-[#1c2128] border border-[#30363d] text-[#8b949e] text-xs font-mono rounded-bl-sm' :
        'bg-[#161b22] border border-[#30363d] text-[#e6edf3] rounded-bl-sm'}`}>
        {!isUser && !isTool && tool_calls && tool_calls.length > 0 && (
          <div className="flex flex-col gap-1 mb-2 pb-2 border-b border-[#30363d]">
            {tool_calls.map((tc) => (
              <div key={tc.id} className="flex items-center gap-1.5 text-[10px] text-[#d2991d]">
                <span>🔧</span>
                <span className="font-mono font-semibold">{tc.function.name}</span>
                <span className="text-[#8b949e] truncate max-w-[200px]">
                  {(() => { try { const o = JSON.parse(tc.function.arguments); const k = Object.keys(o)[0]; return String(k ? o[k] : '').slice(0, 30); } catch { return tc.function.arguments.slice(0, 40); } })()}
                </span>
              </div>
            ))}
          </div>
        )}
        {isTool && <div className="mb-1 text-[10px] text-[#3fb950]">⚡ Result:</div>}
        <div className={`whitespace-pre-wrap ${isTool ? 'text-xs opacity-80' : 'text-[14px]'}`}>
          {isTool && content.length > 500 ? content.slice(0, 500) + `
... (${content.length - 500} more)` : content}
        </div>
      </div>
    </div>
  );
}
