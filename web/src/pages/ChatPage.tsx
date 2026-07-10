import { useState, useRef, useEffect, useCallback, useMemo } from 'react';
import { Card, Input, Button, Select, Tag, Row, Col, Typography, Tooltip, Popover, Radio } from 'antd';
import {
  SendOutlined, SearchOutlined, ToolOutlined, BugOutlined, AuditOutlined,
  FileTextOutlined, StopOutlined, FolderOpenOutlined, FolderOutlined,
  SafetyOutlined, PlusOutlined, SettingOutlined,
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

// Per-agent config in this chat room
interface AgentSlot {
  name: string;
  avatar: string;
  model: string;
  access: 'full' | 'readonly' | 'sandbox';
}

const DEFAULT_AGENT: AgentSlot = { name: 'default', avatar: '🤖', model: 'GPT-4', access: 'full' };

export default function ChatPage({ sessionId, workspace }: Props) {
  const [messages, setMessages] = useState<Msg[]>([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const [streaming, setStreaming] = useState('');
  const [agents, setAgents] = useState<AgentProfile[]>([]);
  const [activeAgent, setActiveAgent] = useState('default');
  // Agent slots in this chat room (each with own model + access)
  const [agentSlots, setAgentSlots] = useState<AgentSlot[]>([DEFAULT_AGENT]);
  const [editingSlot, setEditingSlot] = useState<number | null>(null);
  const [branch, setBranch] = useState('main');
  const [progress, setProgress] = useState({ step: 0, total: 0 });
  const [abortController, setAbortController] = useState<AbortController | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  const workspaceName = workspace.split('/').filter(Boolean).pop() || workspace;

  useEffect(() => {
    getConfig().then(c => {
      // Sync config model to default agent
      setAgentSlots(prev => prev.map((s, i) => i === 0 ? { ...s, model: c.model || s.model } : s));
    }).catch(() => {});
    listAgents().then(a => setAgents(a.agents || [])).catch(() => {});
  }, []);
  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [messages, streaming]);

  const stopGeneration = () => {
    if (abortController) { abortController.abort(); setAbortController(null); }
    setSending(false); setStreaming('');
  };

  const handleSend = useCallback(async () => {
    const text = input.trim();
    if (!text || sending) return;

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

    setProgress({ step: 1, total: 3 });
    const p1 = setTimeout(() => setProgress({ step: 2, total: 3 }), 800);
    const p2 = setTimeout(() => setProgress({ step: 3, total: 3 }), 1600);

    const ac = new AbortController(); setAbortController(ac);
    let fullText = ''; setStreaming('');
    await streamMessage(text, sessionId,
      chunk => { fullText += chunk; setStreaming(fullText); },
      done => {
        clearTimeout(p1); clearTimeout(p2);
        setProgress({ step: 0, total: 0 }); setStreaming(''); setAbortController(null);
        setMessages(prev => [...prev, { role: 'assistant', content: done, agent: activeAgent }]);
        setSending(false);
      },
      err => {
        clearTimeout(p1); clearTimeout(p2);
        setProgress({ step: 0, total: 0 }); setStreaming(''); setSending(false); setAbortController(null);
        setMessages(prev => [...prev, { role: 'assistant', content: `Error: ${err}`, agent: activeAgent }]);
      },
      activeAgent !== 'default' ? activeAgent : undefined,
    );
  }, [input, sending, sessionId, activeAgent]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend(); }
  };

  const addAgentSlot = () => {
    const newSlot: AgentSlot = { name: `agent-${agentSlots.length + 1}`, avatar: '🤖', model: 'GPT-4', access: 'readonly' };
    setAgentSlots(prev => [...prev, newSlot]);
  };

  const updateSlot = (idx: number, updates: Partial<AgentSlot>) => {
    setAgentSlots(prev => prev.map((s, i) => i === idx ? { ...s, ...updates } : s));
  };

  const removeSlot = (idx: number) => {
    if (idx === 0) return; // can't remove default
    setAgentSlots(prev => prev.filter((_, i) => i !== idx));
    if (activeAgent === agentSlots[idx].name) setActiveAgent('default');
  };

  const featureCards = [
    { icon: <SearchOutlined style={{ fontSize: 20, color: '#5e6ad2' }} />, title: 'Explore & Understand Code', desc: 'Analyze codebase, explain logic, find patterns', action: 'Analyze this codebase' },
    { icon: <ToolOutlined style={{ fontSize: 20, color: '#7c5cfc' }} />, title: 'Build New Features', desc: 'Create features, apps, or tools from scratch', action: 'Build a new feature' },
    { icon: <AuditOutlined style={{ fontSize: 20, color: '#27a644' }} />, title: 'Review & Suggest Changes', desc: 'Code review, refactoring, and improvements', action: 'Review my code' },
    { icon: <BugOutlined style={{ fontSize: 20, color: '#d19a00' }} />, title: 'Fix Issues & Failures', desc: 'Debug errors, fix bugs, resolve problems', action: 'Debug this error' },
  ];

  const activeSlot = agentSlots.find(s => s.name === activeAgent) || agentSlots[0];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* Context bar — workspace + agents */}
      <div style={{
        display: 'flex', alignItems: 'center', gap: 8, padding: '4px 16px',
        borderBottom: '1px solid var(--border)', background: 'var(--bg-panel)', flexShrink: 0,
        minHeight: 38,
      }}>
        <FolderOutlined style={{ color: 'var(--accent)', fontSize: 14 }} />
        <Text style={{ fontSize: 12, color: 'var(--text-primary)', fontWeight: 500, maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {workspaceName}
        </Text>
        <Tooltip title="打开位置">
          <Button type="text" size="small" icon={<FolderOpenOutlined />}
            style={{ color: 'var(--text-muted)', fontSize: 13 }} />
        </Tooltip>

        {/* Divider */}
        <div style={{ width: 1, height: 20, background: 'var(--border)', margin: '0 4px' }} />

        {/* Agent slots — click to config */}
        {agentSlots.map((slot, idx) => (
          <Popover
            key={slot.name}
            trigger="click"
            content={
              <div style={{ width: 240 }}>
                <Text strong style={{ fontSize: 12, display: 'block', marginBottom: 10 }}>
                  {slot.avatar} {slot.name}
                </Text>

                <div style={{ marginBottom: 8 }}>
                  <Text type="secondary" style={{ fontSize: 10, display: 'block', marginBottom: 4 }}>模型</Text>
                  <Select
                    size="small"
                    value={slot.model}
                    onChange={v => updateSlot(idx, { model: v })}
                    style={{ width: '100%' }}
                    options={[
                      { value: 'GPT-4', label: 'GPT-4' },
                      { value: 'GPT-4o', label: 'GPT-4o' },
                      { value: 'Claude 3.5', label: 'Claude 3.5 Sonnet' },
                      { value: 'DeepSeek V3', label: 'DeepSeek V3' },
                      { value: 'Gemini Pro', label: 'Gemini Pro' },
                    ]}
                  />
                </div>

                <div style={{ marginBottom: 8 }}>
                  <Text type="secondary" style={{ fontSize: 10, display: 'block', marginBottom: 4 }}>权限</Text>
                  <Radio.Group
                    size="small"
                    value={slot.access}
                    onChange={e => updateSlot(idx, { access: e.target.value })}
                  >
                    <Space direction="vertical" size={4}>
                      <Radio value="full">
                        <Text style={{ fontSize: 11 }}>🔓 完全访问</Text>
                        <Text type="secondary" style={{ fontSize: 9, display: 'block' }}>读/写/执行</Text>
                      </Radio>
                      <Radio value="readonly">
                        <Text style={{ fontSize: 11 }}>📖 只读</Text>
                        <Text type="secondary" style={{ fontSize: 9, display: 'block' }}>仅读取文件</Text>
                      </Radio>
                      <Radio value="sandbox">
                        <Text style={{ fontSize: 11 }}>📦 沙箱</Text>
                        <Text type="secondary" style={{ fontSize: 9, display: 'block' }}>隔离环境，无网络</Text>
                      </Radio>
                    </Space>
                  </Radio.Group>
                </div>

                {idx > 0 && (
                  <Button size="small" danger block onClick={() => removeSlot(idx)} style={{ marginTop: 4 }}>
                    移除此 Agent
                  </Button>
                )}
              </div>
            }
          >
            <div
              onClick={() => setActiveAgent(slot.name)}
              style={{
                display: 'flex', alignItems: 'center', gap: 4,
                padding: '2px 8px', borderRadius: 14, cursor: 'pointer',
                fontSize: 11,
                background: activeAgent === slot.name ? 'var(--accent-dim)' : 'transparent',
                border: activeAgent === slot.name ? '1px solid var(--accent)' : '1px solid transparent',
                color: activeAgent === slot.name ? 'var(--accent)' : 'var(--text-secondary)',
                fontWeight: activeAgent === slot.name ? 500 : 400,
              }}
            >
              <span>{slot.avatar}</span>
              <span>{slot.name}</span>
              <SettingOutlined style={{ fontSize: 9, opacity: 0.5, marginLeft: 2 }} />
            </div>
          </Popover>
        ))}

        {/* Add agent */}
        <Tooltip title="添加 Agent">
          <Button type="text" size="small" icon={<PlusOutlined />} onClick={addAgentSlot}
            style={{ color: 'var(--text-muted)', fontSize: 11, width: 22, height: 22, padding: 0, borderRadius: 11 }} />
        </Tooltip>

        <div style={{ flex: 1 }} />

        {/* Active agent model badge */}
        <Tooltip title={`当前模型: ${activeSlot.model} · 权限: ${activeSlot.access}`}>
          <Tag style={{ margin: 0, fontSize: 10, lineHeight: '18px' }}>
            {activeSlot.model}
          </Tag>
        </Tooltip>

        {/* Progress indicator */}
        {progress.total > 0 && (
          <div style={{
            display: 'flex', alignItems: 'center', gap: 6,
            background: 'var(--bg-active)', padding: '2px 10px', borderRadius: 10,
            fontSize: 11, color: 'var(--accent)',
          }}>
            <div style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--accent)', animation: 'pulse 1.5s infinite' }} />
            第 {progress.step} / {progress.total} 步
          </div>
        )}
      </div>

      {/* Messages OR Welcome */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '16px 24px' }}>
        {messages.length === 0 && !streaming ? (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', gap: 24, paddingBottom: 60 }}>
            <div style={{
              width: 80, height: 80, borderRadius: 20,
              background: 'linear-gradient(135deg, rgba(94,106,210,0.15), rgba(124,92,252,0.1))',
              display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 32,
            }}>🦊</div>
            <Title level={3} style={{ margin: 0, textAlign: 'center', maxWidth: 500 }}>
              What should we build in <span style={{ color: '#5e6ad2' }}>{workspaceName}</span>?
            </Title>
            <Text type="secondary" style={{ textAlign: 'center', maxWidth: 440, fontSize: 13 }}>
              💡 Use <code>@agent-name</code> to invoke a specific agent.
              📋 Try <code>/spec</code> <code>/plan</code> <code>/tasks</code> <code>/implement</code>
            </Text>
            <Row gutter={[12, 12]} style={{ maxWidth: 640, width: '100%' }}>
              {featureCards.map((card, i) => (
                <Col span={12} key={i}>
                  <Card size="small" hoverable onClick={() => setInput(card.action)} style={{ height: '100%', cursor: 'pointer' }}>
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
                      {m.files.map((f, j) => <Tag key={j} icon={<FileTextOutlined />} style={{ borderRadius: 4, fontSize: 10 }}>{f}</Tag>)}
                    </div>
                  )}
                </div>
              </div>
            ))}
            {streaming && (
              <div style={{ display: 'flex', gap: 10 }}>
                <div style={{ width: 28, height: 28, borderRadius: 6, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 14, background: 'var(--bg-elevated)' }}>🤖</div>
                <div style={{ flex: 1 }}>
                  <Text type="secondary" style={{ fontSize: 10, fontWeight: 600, textTransform: 'uppercase' }}>
                    Codex <Tag color="processing" style={{ fontSize: 9, marginLeft: 4 }}>Writing...</Tag>
                  </Text>
                  <div style={{ fontSize: 13, lineHeight: 1.65, whiteSpace: 'pre-wrap' }}><Markdown text={streaming} /></div>
                </div>
              </div>
            )}
            <div ref={bottomRef} />
          </div>
        )}
      </div>

      {/* Input area — chat room style */}
      <div style={{ padding: '8px 24px 10px', borderTop: '1px solid var(--border)', background: 'var(--bg-root)' }}>
        <div style={{ maxWidth: 720, margin: '0 auto' }}>
          {/* Main input row */}
          <div style={{
            display: 'flex', alignItems: 'flex-end', gap: 8,
            background: 'var(--bg-panel)', border: '1px solid var(--border)',
            borderRadius: 10, padding: '6px 8px',
          }}>
            <Tooltip title="添加上下文">
              <Button type="text" size="small" icon={<span style={{ fontWeight: 700, fontSize: 16 }}>+</span>}
                style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
            </Tooltip>

            <TextArea
              value={input}
              onChange={e => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="要求后续变更"
              autoSize={{ minRows: 1, maxRows: 5 }}
              disabled={sending}
              style={{
                flex: 1, border: 'none', background: 'transparent',
                resize: 'none', outline: 'none', padding: '4px 0',
                boxShadow: 'none',
              }}
              className="chat-input-textarea"
            />

            {sending ? (
              <Button type="text" size="small" icon={<StopOutlined />} onClick={stopGeneration}
                style={{ width: 30, height: 30, borderRadius: 15, background: 'var(--bg-elevated)', color: 'var(--text-secondary)', flexShrink: 0 }} />
            ) : (
              <Button type="primary" icon={<SendOutlined />} onClick={handleSend} disabled={!input.trim()}
                style={{ width: 30, height: 30, borderRadius: 15, flexShrink: 0, padding: 0 }} />
            )}
          </div>

          {/* Bottom toolbar: Branch only */}
          <div style={{
            display: 'flex', alignItems: 'center', gap: 8, marginTop: 6,
            padding: '0 4px',
          }}>
            <Select
              size="small"
              value={branch}
              onChange={setBranch}
              style={{ minWidth: 110 }}
              options={[
                { value: 'main', label: '🌿 main' },
                { value: 'develop', label: '🌿 develop' },
                { value: 'feature/ai-agent', label: '🌿 feature/ai-agent' },
              ]}
            />

            <div style={{ flex: 1 }} />

            <Text type="secondary" style={{ fontSize: 10, color: 'var(--text-muted)' }}>
              {activeSlot.model} · {activeSlot.access === 'full' ? '完全访问' : activeSlot.access === 'readonly' ? '只读' : '沙箱'}
            </Text>
          </div>
        </div>
      </div>

      <style>{`
        @keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.4} }
        .chat-input-textarea { box-shadow: none !important; }
        .chat-input-textarea:focus { box-shadow: none !important; }
      `}</style>
    </div>
  );
}

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

  function renderText(plain: string): React.ReactNode[] {
    const pathRe = /([A-Z]:\\[\w\-\\]+\.[\w]+)/g;
    const elements: React.ReactNode[] = [];
    let lastIdx = 0;
    let match: RegExpExecArray | null;
    while ((match = pathRe.exec(plain)) !== null) {
      if (match.index > lastIdx) elements.push(plain.slice(lastIdx, match.index));
      elements.push(
        <Tag key={match.index} style={{
          background: 'var(--bg-elevated)', border: '1px solid var(--border)',
          borderRadius: 4, fontSize: 10, fontFamily: "'JetBrains Mono', monospace",
          padding: '1px 6px', lineHeight: 1.6,
        }}>
          <FileTextOutlined style={{ marginRight: 3 }} />
          {match[1].split('\\').slice(-2).join('\\')}
        </Tag>
      );
      lastIdx = match.index + match[0].length;
    }
    if (lastIdx < plain.length) elements.push(plain.slice(lastIdx));
    return elements;
  }

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
    ) : <span key={i}>{renderText(b.content)}</span>)}
  </>;
}
