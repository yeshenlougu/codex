import { useState, useRef, useEffect, useCallback, useMemo } from 'react';
import { Card, Input, Button, Select, Tag, Empty, Row, Col, Typography } from 'antd';
import {
  SendOutlined, SearchOutlined, ToolOutlined, BugOutlined, AuditOutlined,
  FileTextOutlined, FolderOutlined,
} from '@ant-design/icons';
import { streamMessage, getConfig, listAgents, getTasks, implementTask } from '../lib/api';
import type { AgentProfile } from '../lib/types';

const { Text, Title } = Typography;
const { TextArea } = Input;

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

    // Slash commands
    if (text.startsWith('/tasks')) {
      setSending(true); setInput('');
      setMessages(prev => [...prev, { role: 'user', content: text }]);
      try { const res = await getTasks(); setMessages(prev => [...prev, { role: 'assistant', content: res.content || 'No tasks.' }]); }
      catch (e: any) { setMessages(prev => [...prev, { role: 'assistant', content: `Error: ${e.message}` }]); }
      setSending(false); return;
    }
    if (text.startsWith('/implement')) {
      const m = text.match(/^\/implement\s+(\d+)/);
      if (!m) { setMessages(prev => [...prev, { role: 'user', content: text }, { role: 'assistant', content: 'Usage: /implement <task-number>' }]); setInput(''); return; }
      setSending(true); setInput('');
      setMessages(prev => [...prev, { role: 'user', content: text }]);
      try { const res = await implementTask(parseInt(m[1])); setMessages(prev => [...prev, { role: 'assistant', content: res.content }]); }
      catch (e: any) { setMessages(prev => [...prev, { role: 'assistant', content: `Error: ${e.message}` }]); }
      setSending(false); return;
    }

    setInput(''); setSending(true);
    const fileMatches = text.match(/[\w./-]+\.\w{1,6}/g) || [];
    setMessages(prev => [...prev, { role: 'user', content: text, files: fileMatches }]);
    let fullText = ''; setStreaming('');
    await streamMessage(text, sessionId,
      chunk => { fullText += chunk; setStreaming(fullText); },
      done => {
        setStreaming('');
        setMessages(prev => [...prev, { role: 'assistant', content: done, agent: activeAgent }]);
        setSending(false);
      },
      err => { setStreaming(''); setSending(false); setMessages(prev => [...prev, { role: 'assistant', content: `Error: ${err}`, agent: activeAgent }]); },
      activeAgent !== 'default' ? activeAgent : undefined,
    );
  }, [input, sending, sessionId, activeAgent]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend(); }
  };

  // Feature cards for welcome state
  const featureCards = [
    { icon: <SearchOutlined style={{ fontSize: 20, color: '#5e6ad2' }} />, title: 'Explore & Understand Code', desc: 'Analyze codebase, explain logic, find patterns', action: 'Analyze this codebase' },
    { icon: <ToolOutlined style={{ fontSize: 20, color: '#7c5cfc' }} />, title: 'Build New Features', desc: 'Create features, apps, or tools from scratch', action: 'Build a new feature' },
    { icon: <AuditOutlined style={{ fontSize: 20, color: '#27a644' }} />, title: 'Review & Suggest Changes', desc: 'Code review, refactoring, and improvements', action: 'Review my code' },
    { icon: <BugOutlined style={{ fontSize: 20, color: '#d19a00' }} />, title: 'Fix Issues & Failures', desc: 'Debug errors, fix bugs, resolve problems', action: 'Debug this error' },
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* Context bar */}
      <div style={{
        display: 'flex', alignItems: 'center', gap: 12, padding: '4px 16px',
        borderBottom: '1px solid var(--border)', background: 'var(--bg-panel)', flexShrink: 0,
      }}>
        <Tag color="green" style={{ margin: 0 }}>{provider} / {model}</Tag>
        <Text type="secondary" style={{ fontSize: 11 }}>📁 {workspace}</Text>
        <Text type="secondary" style={{ fontSize: 10, marginLeft: 'auto', fontFamily: 'monospace' }}>#{sessionId.slice(-12)}</Text>
        <Select size="small" value={activeAgent} onChange={setActiveAgent} style={{ width: 160 }}
          options={[{ value: 'default', label: '🤖 default' }, ...agents.map(a => ({ value: a.name, label: `${a.avatar || '🤖'} ${a.name}` }))]}
        />
      </div>

      {/* Messages OR Welcome */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '16px 24px' }}>
        {messages.length === 0 && !streaming ? (
          /* Welcome page */
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', gap: 24, paddingBottom: 60 }}>
            <div style={{
              width: 80, height: 80, borderRadius: 20,
              background: 'linear-gradient(135deg, rgba(94,106,210,0.15), rgba(124,92,252,0.1))',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 32,
            }}>🦊</div>
            <Title level={3} style={{ margin: 0, textAlign: 'center', maxWidth: 500 }}>
              What should we build in <span style={{ color: '#5e6ad2' }}>{workspace}</span>?
            </Title>
            <Text type="secondary" style={{ textAlign: 'center', maxWidth: 440, fontSize: 13 }}>
              💡 Use <code>@agent-name</code> to invoke a specific agent.
              📋 Try <code>/spec</code> <code>/plan</code> <code>/tasks</code> <code>/implement</code>
            </Text>

            {/* Feature cards grid */}
            <Row gutter={[12, 12]} style={{ maxWidth: 640, width: '100%' }}>
              {featureCards.map((card, i) => (
                <Col span={12} key={i}>
                  <Card
                    size="small"
                    hoverable
                    onClick={() => setInput(card.action)}
                    style={{ height: '100%', cursor: 'pointer' }}
                  >
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                      {card.icon}
                      <Text strong style={{ fontSize: 13 }}>{card.title}</Text>
                      <Text type="secondary" style={{ fontSize: 11, lineHeight: 1.4 }}>{card.desc}</Text>
                    </div>
                  </Card>
                </Col>
              ))}
            </Row>
          </div>
        ) : (
          /* Messages */
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            {messages.map((m, i) => (
              <div key={i} style={{ display: 'flex', gap: 10 }}>
                <div style={{
                  width: 28, height: 28, borderRadius: 6, display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontSize: 14, flexShrink: 0,
                  background: m.role === 'user' ? 'var(--accent-dim)' : 'var(--bg-elevated)',
                }}>
                  {m.role === 'user' ? '👤' : '🤖'}
                </div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <Text type="secondary" style={{ fontSize: 10, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em', display: 'block', marginBottom: 2 }}>
                    {m.role === 'user' ? 'You' : (m.agent || 'Codex')}
                  </Text>
                  <div style={{ fontSize: 13, lineHeight: 1.65, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                    <Markdown text={m.content} />
                  </div>
                  {m.files && m.files.length > 0 && (
                    <div style={{ marginTop: 6, display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                      {m.files.map((f, j) => <Tag key={j} icon={<FileTextOutlined />}>{f}</Tag>)}
                    </div>
                  )}
                </div>
              </div>
            ))}
            {streaming && (
              <div style={{ display: 'flex', gap: 10 }}>
                <div style={{ width: 28, height: 28, borderRadius: 6, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 14, background: 'var(--bg-elevated)' }}>🤖</div>
                <div style={{ flex: 1 }}>
                  <Text type="secondary" style={{ fontSize: 10, fontWeight: 600, textTransform: 'uppercase' }}>Codex <Tag color="processing" style={{ fontSize: 9, marginLeft: 4 }}>Writing...</Tag></Text>
                  <div style={{ fontSize: 13, lineHeight: 1.65, whiteSpace: 'pre-wrap' }}><Markdown text={streaming} /></div>
                </div>
              </div>
            )}
            <div ref={bottomRef} />
          </div>
        )}
      </div>

      {/* Input */}
      <div style={{ padding: '10px 24px 14px', borderTop: '1px solid var(--border)', background: 'var(--bg-root)' }}>
        <div style={{ maxWidth: 720, margin: '0 auto', display: 'flex', gap: 8, alignItems: 'flex-end' }}>
          <TextArea
            value={input}
            onChange={e => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={`Ask ${activeAgent}... (${workspace})`}
            autoSize={{ minRows: 1, maxRows: 5 }}
            disabled={sending}
            style={{ flex: 1 }}
          />
          <Button
            type="primary"
            icon={<SendOutlined />}
            onClick={handleSend}
            disabled={sending || !input.trim()}
            style={{ height: 40 }}
          />
        </div>
      </div>
    </div>
  );
}

// Simple markdown renderer (code blocks only)
function Markdown({ text }: { text: string }) {
  const blocks = useMemo(() => {
    const parts: { type: 'text' | 'code'; content: string; lang?: string }[] = [];
    const re = /```(\w*)\n([\s\S]*?)```/g;
    let last = 0, m: RegExpExecArray | null;
    while ((m = re.exec(text)) !== null) {
      if (m.index > last) parts.push({ type: 'text', content: text.slice(last, m.index) });
      parts.push({ type: 'code', lang: m[1], content: m[2].trimEnd() });
      last = m.index + m[0].length;
    }
    if (last < text.length) parts.push({ type: 'text', content: text.slice(last) });
    return parts;
  }, [text]);

  return <>
    {blocks.map((b, i) => b.type === 'code' ? (
      <pre key={i} style={{
        background: 'var(--bg-panel)', border: '1px solid var(--border)', borderRadius: 8,
        padding: '10px 14px', margin: '6px 0', overflow: 'auto', fontSize: 12,
        fontFamily: "'JetBrains Mono', monospace",
      }}>
        {b.lang && <div style={{ fontSize: 10, color: 'var(--text-muted)', marginBottom: 4 }}>{b.lang}</div>}
        <code>{b.content}</code>
      </pre>
    ) : <span key={i}>{b.content}</span>)}
  </>;
}
