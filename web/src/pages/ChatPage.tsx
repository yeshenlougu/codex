import { useState, useRef, useEffect, useCallback, useMemo } from 'react';
import { streamMessage, getConfig, listAgents, getTasks, implementTask } from '../lib/api';
import type { Task } from '../lib/api';
import type { AgentProfile } from '../lib/types';

interface Props {
  sessionId: string;
  workspace: string;
}

interface Msg { role: 'user' | 'assistant'; content: string; files?: string[]; agent?: string }

export default function ChatPage({ sessionId, workspace }: Props) {
  const [messages, setMessages] = useState<Msg[]>([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const [streaming, setStreaming] = useState('');
  const [provider, setProvider] = useState('openai');
  const [model, setModel] = useState('gpt-4');
  const [agents, setAgents] = useState<AgentProfile[]>([]);
  const [activeAgent, setActiveAgent] = useState('default');
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    getConfig().then(c => { setProvider(c.provider); setModel(c.model); }).catch(() => {});
    listAgents().then(a => setAgents(a.agents || [])).catch(() => {});
  }, []);
  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [messages, streaming]);

  const handleSend = useCallback(async () => {
    const text = input.trim();
    if (!text || sending) return;

    // ---- Slash command detection ----
    if (text.startsWith('/tasks')) {
      setSending(true);
      setInput('');
      setMessages((prev) => [...prev, { role: 'user', content: text }]);
      try {
        const res = await getTasks();
        setMessages((prev) => [...prev, { role: 'assistant', content: res.content || 'No tasks found.' }]);
      } catch (err: any) {
        setMessages((prev) => [...prev, { role: 'assistant', content: `Error: ${err.message}` }]);
      }
      setSending(false);
      return;
    }

    if (text.startsWith('/implement')) {
      const match = text.match(/^\/implement\s+(\d+)/);
      if (!match) {
        setInput('');
        setMessages((prev) => [
          ...prev,
          { role: 'user', content: text },
          { role: 'assistant', content: 'Usage: /implement <task-number>\nExample: /implement 3' },
        ]);
        return;
      }
      const taskNum = parseInt(match[1], 10);
      setSending(true);
      setInput('');
      setMessages((prev) => [...prev, { role: 'user', content: text }]);
      try {
        const res = await implementTask(taskNum);
        setMessages((prev) => [...prev, { role: 'assistant', content: res.content }]);
      } catch (err: any) {
        setMessages((prev) => [...prev, { role: 'assistant', content: `Error: ${err.message}` }]);
      }
      setSending(false);
      return;
    }

    // /spec and /plan go to chat (backend intercepts them)
    setInput('');
    setSending(true);
    const fileMatches = text.match(/[\w./-]+\.\w{1,6}/g) || [];
    setMessages((prev) => [...prev, { role: 'user', content: text, files: fileMatches }]);

    let fullText = '';
    setStreaming('');

    await streamMessage(
      text, sessionId,
      (chunk) => { fullText += chunk; setStreaming(fullText); },
      (done) => {
        setStreaming('');
        const respFiles = [...new Set([...fileMatches, ...extractFiles(done)])];
        setMessages((prev) => [...prev, { role: 'assistant', content: done, files: respFiles, agent: activeAgent }]);
        setSending(false);
      },
      (err) => {
        setStreaming('');
        setMessages((prev) => [...prev, { role: 'assistant', content: `Error: ${err}`, agent: activeAgent }]);
        setSending(false);
      },
      activeAgent !== 'default' ? activeAgent : undefined
    );
  }, [input, sending, sessionId, activeAgent]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend(); }
  };

  return (
    <div className="chat-container">
      {/* Context bar */}
      <div className="chat-context-bar">
        <span className="ctx-model"><span className="ctx-dot" /> {provider} / {model}</span>
        <span className="ctx-workspace">📁 {workspace}</span>
        <span className="ctx-session">#{sessionId.slice(-12)}</span>
        {/* Agent selector */}
        <select
          className="ctx-agent-select"
          value={activeAgent}
          onChange={(e) => setActiveAgent(e.target.value)}
          title="Select active agent"
        >
          {agents.length === 0 && <option value="default">🤖 default</option>}
          {agents.map(a => (
            <option key={a.name} value={a.name}>{a.avatar || '🤖'} {a.name}{a.is_builtin ? ' (built-in)' : ''}</option>
          ))}
        </select>
      </div>

      {/* Messages */}
      <div className="chat-messages">
        {messages.length === 0 && !streaming ? (
          <div className="chat-empty">
            <img src="/assets/mascot.png" alt="Codex" className="chat-empty-mascot" />
            <h2>Codex Go</h2>
            <p>Your AI coding companion. Read, write, edit code.<br />Explain, refactor, debug — all in one place.</p>
            <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 8 }}>
              💡 Use <code>@agent-name</code> in your message to invoke a specific agent.
            </p>
            <p style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 4 }}>
              📋 Slash commands: <code>/spec</code> <code>/plan</code> <code>/tasks</code> <code>/implement</code>
            </p>
            <div className="chat-empty-hints">
              <span onClick={() => setInput('/spec Add dark mode support')}>📝 /spec new feature</span>
              <span onClick={() => setInput('/plan')}>📋 /plan from spec</span>
              <span onClick={() => setInput('/tasks')}>✅ /tasks view</span>
              <span onClick={() => setInput('/implement 1')}>✔️ /implement task</span>
            </div>
          </div>
        ) : (
          messages.map((m, i) => <MessageBlock key={i} msg={m} />)
        )}
        {streaming && <MessageBlock msg={{ role: 'assistant', content: streaming, agent: activeAgent }} streaming />}
        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <div className="chat-input-area">
        <div className="chat-input-row">
          <textarea
            className="chat-input"
            value={input}
            onChange={(e) => { setInput(e.target.value); const t = e.target; t.style.height = 'auto'; t.style.height = Math.min(t.scrollHeight, 160) + 'px'; }}
            onKeyDown={handleKeyDown}
            placeholder={`Ask ${activeAgent}... (${workspace})`}
            rows={1}
            disabled={sending}
          />
          <button className="chat-send-btn" onClick={handleSend} disabled={sending || !input.trim()}>
            {sending ? '◉' : '→'}
          </button>
        </div>
      </div>
    </div>
  );
}

// ============ Message Block ============

function MessageBlock({ msg, streaming }: { msg: Msg; streaming?: boolean }) {
  const isUser = msg.role === 'user';
  const avatar = isUser ? '👤' : (msg.agent === 'default' ? '🤖' : '🤖');
  const label = isUser ? 'You' : (msg.agent || 'Codex');

  return (
    <div className={`msg-block ${isUser ? 'msg-user' : 'msg-assistant'} ${streaming ? 'msg-streaming' : ''}`}>
      <div className="msg-avatar">{avatar}</div>
      <div className="msg-body">
        <div className="msg-header">
          <span className="msg-role">{label}</span>
          {streaming && <span className="msg-thinking">Writing...</span>}
        </div>
        <div className="msg-content"><MarkdownContent text={msg.content} /></div>
        {msg.files && msg.files.length > 0 && (
          <div className="msg-files">
            {msg.files.map((f, i) => <span key={i} className="file-chip">📄 {f}</span>)}
          </div>
        )}
      </div>
    </div>
  );
}

// ============ Markdown / Code Renderer ============

function MarkdownContent({ text }: { text: string }) {
  const blocks = useMemo(() => parseBlocks(text), [text]);
  return <>{blocks.map((b, i) => b.type === 'code'
    ? <CodeBlock key={i} code={b.content} language={b.lang || 'text'} />
    : <span key={i} className="md-text">{b.content}</span>
  )}</>;
}

interface Block { type: 'text' | 'code'; content: string; lang?: string }

function parseBlocks(text: string): Block[] {
  const blocks: Block[] = [];
  const regex = /```(\w*)\n([\s\S]*?)```/g;
  let lastIdx = 0, match: RegExpExecArray | null;
  while ((match = regex.exec(text)) !== null) {
    if (match.index > lastIdx) blocks.push({ type: 'text', content: text.slice(lastIdx, match.index) });
    blocks.push({ type: 'code', lang: match[1], content: match[2].trimEnd() });
    lastIdx = match.index + match[0].length;
  }
  if (lastIdx < text.length) blocks.push({ type: 'text', content: text.slice(lastIdx) });
  return blocks;
}

function CodeBlock({ code, language }: { code: string; language: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = () => { navigator.clipboard.writeText(code).then(() => { setCopied(true); setTimeout(() => setCopied(false), 2000); }); };
  return (
    <div className="code-block">
      <div className="code-block-header">
        <span className="code-lang">{language}</span>
        <button className="code-copy-btn" onClick={handleCopy}>{copied ? '✓ Copied' : '📋 Copy'}</button>
      </div>
      <pre className="code-block-pre"><code>
        {code.split('\n').map((line, i) => (
          <div key={i} className="code-line">
            <span className="code-ln">{i + 1}</span>
            <span className="code-text">{line || ' '}</span>
          </div>
        ))}
      </code></pre>
    </div>
  );
}

// ============ Helpers ============

function extractFiles(text: string): string[] {
  const re = /[\w./-]+\.\w{1,6}/g;
  const matches: string[] = text.match(re) || [];
  return [...new Set(matches.filter(f => !f.startsWith('http') && f.includes('.')))].slice(0, 10);
}
