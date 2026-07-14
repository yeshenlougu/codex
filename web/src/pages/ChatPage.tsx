import { useState, useRef, useEffect, useCallback } from 'react';
import { Card, Input, Button, Tag, Row, Col, Typography, Tooltip, Popover, Modal, Space, Result, Dropdown } from 'antd';
import {
  SendOutlined, SearchOutlined, ToolOutlined, BugOutlined, AuditOutlined,
  FileTextOutlined, StopOutlined, PlusOutlined, SettingOutlined,
  EditOutlined, DeleteOutlined, BulbOutlined, RocketOutlined,
  PlayCircleOutlined, CheckCircleOutlined, CloseCircleOutlined,
} from '@ant-design/icons';
import { streamMessage, getConfig, listAgents, getTasks, implementTask, executeTask, approveCheck } from '../lib/api';
import type { AgentProfile } from '../lib/types';
import type { Page } from '../App';

const { Text, Title } = Typography;
const { TextArea } = Input;

interface Props {
  sessionId: string;
  workspace: string;
  onNavigate: (p: Page) => void;
}

interface Msg { role: 'user' | 'assistant'; content: string; files?: string[]; agent?: string }

interface QueuedItem {
  id: string;
  content: string;
  mode: 'direct' | 'steer';
  expanded: boolean;
  // steer mode sub-steps — sent as /spec /plan /tasks /implement
  steerSent: { spec: boolean; plan: boolean; tasks: boolean; implement: boolean };
}

const STEER_STEPS = [
  { key: 'spec' as const, label: '/spec', desc: 'Spec', color: '#5e6ad2' },
  { key: 'plan' as const, label: '/plan', desc: 'Plan', color: '#7c5cfc' },
  { key: 'tasks' as const, label: '/tasks', desc: 'Tasks', color: '#27a644' },
  { key: 'implement' as const, label: '/implement', desc: 'Implement', color: '#d19a00' },
  { key: 'execute' as const, label: '/execute', desc: 'Execute', color: '#e85347' },
];

interface ApprovalCheck {
  id: number; tool: string; args: string; risk: string; description: string;
}

export default function ChatPage({ sessionId, workspace, onNavigate }: Props) {
  const [messages, setMessages] = useState<Msg[]>([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const [streaming, setStreaming] = useState('');
  const [agents, setAgents] = useState<AgentProfile[]>([]);
  const [activeAgent, setActiveAgent] = useState('default');
  const [ballOpen, setBallOpen] = useState(false);
  const [progress, setProgress] = useState({ step: 0, total: 0 });
  const [abortController, setAbortController] = useState<AbortController | null>(null);
  const [queue, setQueue] = useState<QueuedItem[]>([]);
  const [pendingApproval, setPendingApproval] = useState<ApprovalCheck | null>(null);
  const [activeExecTask, setActiveExecTask] = useState<number | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  // WebSocket for approval notifications
  useEffect(() => {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${proto}//${location.host}/ws`);
    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data);
        if (msg.type === 'approval_request') {
          const check: ApprovalCheck = typeof msg.content === 'string'
            ? JSON.parse(msg.content) : msg;
          setPendingApproval(check);
        }
      } catch { /* ignore */ }
    };
    ws.onclose = () => {}; // auto-reconnect on next message
    return () => { ws.close(); };
  }, []);

  const workspaceName = workspace.split('/').filter(Boolean).pop() || workspace;

  useEffect(() => {
    getConfig().catch(() => {});
    listAgents().then(a => setAgents(a.agents || [])).catch(() => {});
  }, []);

  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [messages, streaming]);

  const stopGeneration = () => {
    if (abortController) { abortController.abort(); setAbortController(null); }
    setSending(false); setStreaming('');
  };

  // Execute a task via SSE agent (defined before sendOne so it's available)
  const execOne = useCallback(async (taskNum: number) => {
    setSending(true);
    setActiveExecTask(taskNum);
    let streamingText = '';
    setStreaming('');
    await executeTask(taskNum, sessionId, activeAgent !== 'default' ? activeAgent : undefined,
      chunk => { streamingText += chunk; setStreaming(streamingText); },
      result => {
        setStreaming('');
        setActiveExecTask(null);
        setMessages(prev => [...prev, { role: 'assistant' as const, content: result || streamingText, agent: activeAgent }]);
        setSending(false);
      },
      err => {
        setStreaming('');
        setActiveExecTask(null);
        setSending(false);
        setMessages(prev => [...prev, { role: 'assistant' as const, content: `❌ Execution error: ${err}`, agent: activeAgent }]);
      },
    );
  }, [sessionId, activeAgent]);

  // Send a single message (called from queue or direct)
  const sendOne = useCallback(async (text: string) => {
    const fileMatches = text.match(/[\w./-]+\.\w{1,6}/g) || [];
    setMessages(prev => [...prev, { role: 'user', content: text, files: fileMatches }]);

    if (text.startsWith('/tasks')) {
      const res = await getTasks();
      setMessages(prev => [...prev, { role: 'assistant', content: res.content || 'No tasks.' }]);
      return;
    }
    if (text.startsWith('/implement')) {
      const m = text.match(/^\/implement\s+(\d+)/);
      if (!m) { setMessages(prev => [...prev, { role: 'assistant', content: 'Usage: /implement <task-number>' }]); return; }
      const res = await implementTask(parseInt(m[1]));
      setMessages(prev => [...prev, { role: 'assistant', content: res.content }]);
      return;
    }
    if (text.startsWith('/execute')) {
      const m = text.match(/^\/execute\s+(\d+)/);
      if (!m) { setMessages(prev => [...prev, { role: 'assistant', content: 'Usage: /execute <task-number> — runs the task through the agent' }]); return; }
      execOne(parseInt(m[1]));
      return;
    }

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
  }, [sessionId, activeAgent, execOne]);

  // Called from queue — send then remove from queue
  const sendFromQueue = useCallback(async (item: QueuedItem) => {
    setQueue(prev => prev.filter(q => q.id !== item.id));
    setSending(true);
    // Steer mode: wrap in /steer command, let backend handle the full workflow
    const text = item.mode === 'steer' ? `/steer ${item.content}` : item.content;
    await sendOne(text);
    setSending(false);
  }, [sendOne]);

  // Send a single steer step from queue
  const sendSteerStep = useCallback(async (item: QueuedItem, step: string) => {
    let cmd: string;
    switch (step) {
      case 'spec': cmd = '/spec ' + item.content; break;
      case 'plan': cmd = '/plan'; break;  // no arg — backend auto-discovers spec file
      case 'tasks': cmd = '/tasks'; break;
      case 'implement': cmd = '/implement 1'; break;  // default task 1; user can change
      case 'execute': cmd = '/execute 1'; break;
      default: cmd = '/' + step + ' ' + item.content;
    }
    setSending(true);
    await sendOne(cmd);
    setQueue(prev => prev.map(q => q.id === item.id ? {
      ...q, steerSent: { ...q.steerSent, [step]: true }
    } : q));
    setSending(false);
  }, [sendOne]);

  const handleSend = useCallback(() => {
    const text = input.trim();
    if (!text || sending) return;
    setInput('');

    // Welcome page: send directly
    if (messages.length === 0) {
      setSending(true);
      sendOne(text).finally(() => setSending(false));
      return;
    }

    // In conversation: add to queue
    const qid = Date.now().toString(36);
    setQueue(prev => [...prev, {
      id: qid, content: text, mode: 'direct' as const, expanded: false,
      steerSent: { spec: false, plan: false, tasks: false, implement: false },
    }]);
  }, [input, sending, messages.length, sendOne]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend(); }
  };

  // Queue actions
  const toggleSteer = (id: string) => setQueue(prev => prev.map(q =>
    q.id === id ? { ...q, mode: q.mode === 'steer' ? 'direct' : 'steer', expanded: q.mode !== 'steer' } : q
  ));
  const toggleExpand = (id: string) => setQueue(prev => prev.map(q =>
    q.id === id ? { ...q, expanded: !q.expanded } : q
  ));
  const editItem = (item: QueuedItem) => { setInput(item.content); setQueue(prev => prev.filter(q => q.id !== item.id)); };
  const removeItem = (id: string) => setQueue(prev => prev.filter(q => q.id !== id));
  const handleApprove = async (approved: boolean) => {
    if (!pendingApproval) return;
    try { await approveCheck(pendingApproval.id, approved); } catch {}
    setPendingApproval(null);
  };

  const featureCards = [
    { icon: <SearchOutlined style={{ fontSize: 20, color: '#5e6ad2' }} />, title: 'Explore & Understand Code', desc: 'Analyze codebase, explain logic, find patterns', action: 'Analyze this codebase' },
    { icon: <ToolOutlined style={{ fontSize: 20, color: '#7c5cfc' }} />, title: 'Build New Features', desc: 'Create features, apps, or tools from scratch', action: 'Build a new feature' },
    { icon: <AuditOutlined style={{ fontSize: 20, color: '#27a644' }} />, title: 'Review & Suggest Changes', desc: 'Code review, refactoring, and improvements', action: 'Review my code' },
    { icon: <BugOutlined style={{ fontSize: 20, color: '#d19a00' }} />, title: 'Fix Issues & Failures', desc: 'Debug errors, fix bugs, resolve problems', action: 'Debug this error' },
  ];

  const activeAgentProfile = agents.find(a => a.name === activeAgent);
  const agentAvatar = activeAgentProfile?.name === 'default' ? '🤖' : '🤖';

  // Build default + custom agents list
  const allAgentNames = ['default', ...agents.filter(a => a.name !== 'default').map(a => a.name)];

  const agentListContent = (
    <div style={{ width: 200 }}>
      <Text type="secondary" style={{ fontSize: 10, textTransform: 'uppercase', letterSpacing: '0.08em', display: 'block', marginBottom: 8 }}>
        当前聊天室 Agent
      </Text>
      {allAgentNames.map(name => {
        const isActive = activeAgent === name;
        const profile = agents.find(a => a.name === name);
        return (
          <div
            key={name}
            style={{
              display: 'flex', alignItems: 'center', gap: 8,
              padding: '6px 8px', borderRadius: 6, cursor: 'pointer', marginBottom: 2,
              background: isActive ? 'var(--accent-dim)' : 'transparent',
              transition: 'background 0.1s',
            }}
          >
            <div
              onClick={() => { setActiveAgent(name); setBallOpen(false); }}
              style={{ display: 'flex', alignItems: 'center', gap: 6, flex: 1, minWidth: 0 }}
            >
              <span style={{ fontSize: 16 }}>{agentAvatar}</span>
              <div style={{ overflow: 'hidden' }}>
                <div style={{ fontSize: 12, fontWeight: isActive ? 600 : 400, color: isActive ? 'var(--accent)' : 'var(--text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {name}
                </div>
                {profile && (
                  <Text type="secondary" style={{ fontSize: 9 }}>
                    {profile.model?.model || 'GPT-4'}
                  </Text>
                )}
              </div>
            </div>
            <Tooltip title="在设置中配置">
              <Button
                type="text" size="small"
                icon={<SettingOutlined />}
                style={{ color: 'var(--text-muted)', fontSize: 12, flexShrink: 0 }}
                onClick={(e) => { e.stopPropagation(); setBallOpen(false); onNavigate('settings'); }}
              />
            </Tooltip>
          </div>
        );
      })}
      <div style={{ height: 1, background: 'var(--border)', margin: '6px 0' }} />
      <Button
        type="text" size="small" block
        icon={<PlusOutlined />}
        onClick={() => { setBallOpen(false); onNavigate('settings'); }}
        style={{ fontSize: 11, color: 'var(--text-secondary)', justifyContent: 'flex-start' }}
      >
        添加 Agent
      </Button>
    </div>
  );

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', position: 'relative' }}>
      {/* Messages OR Welcome */}
      <div style={{ flex: 1, overflowY: 'auto', padding: messages.length > 0 ? '16px 24px' : '0 24px' }}>
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
              📋 Try <code>/spec</code> <code>/plan</code> <code>/tasks</code> <code>/implement</code> <code>/execute</code>
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
                    {m.role === 'user' ? 'You' : (m.agent || agentAvatar)}
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

      {/* Queue Bar — appears when conversation is in progress */}
      {queue.length > 0 && (
        <div style={{
          borderTop: '1px solid var(--border)', background: 'var(--bg-panel)',
          padding: '6px 16px', maxHeight: 200, overflowY: 'auto', flexShrink: 0,
        }}>
          <Text type="secondary" style={{ fontSize: 10, textTransform: 'uppercase', letterSpacing: '0.08em', display: 'block', marginBottom: 6 }}>
            队列 ({queue.length})
          </Text>
          {queue.map(item => (
            <div key={item.id}>
              <div style={{
                display: 'flex', alignItems: 'center', gap: 6,
                padding: '4px 8px', borderRadius: 6, marginBottom: 2,
                background: 'var(--bg-root)', border: '1px solid var(--border)',
              }}>
                <Text style={{
                  flex: 1, fontSize: 12, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                  color: 'var(--text-secondary)',
                }}>
                  {item.mode === 'steer' ? '/steer ' : ''}{item.content}
                </Text>
                {item.mode === 'steer' && (
                  <Tag color="purple" style={{ margin: 0, fontSize: 9, lineHeight: '16px' }}>/steer</Tag>
                )}
                <Tooltip title="发送">
                  <Button type="text" size="small" icon={<SendOutlined />}
                    onClick={() => sendFromQueue(item)} disabled={sending}
                    style={{ color: 'var(--accent)', fontSize: 12, width: 26, height: 26, padding: 0 }} />
                </Tooltip>
                <Tooltip title={item.mode === 'steer' ? '取消引导' : '引导模式'}>
                  <Button type="text" size="small" icon={<BulbOutlined />}
                    onClick={() => toggleSteer(item.id)}
                    style={{ color: item.mode === 'steer' ? '#7c5cfc' : 'var(--text-muted)', fontSize: 12, width: 26, height: 26, padding: 0 }} />
                </Tooltip>
                <Tooltip title="编辑">
                  <Button type="text" size="small" icon={<EditOutlined />}
                    onClick={() => editItem(item)}
                    style={{ color: 'var(--text-muted)', fontSize: 12, width: 26, height: 26, padding: 0 }} />
                </Tooltip>
                <Tooltip title="删除">
                  <Button type="text" size="small" icon={<DeleteOutlined />}
                    onClick={() => removeItem(item.id)}
                    style={{ color: 'var(--text-muted)', fontSize: 12, width: 26, height: 26, padding: 0 }} />
                </Tooltip>
              </div>
              {/* Steer sub-steps */}
              {item.mode === 'steer' && (
                <div style={{ margin: '2px 0 6px 12px', display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                  {STEER_STEPS.map(s => (
                    <Tooltip key={s.key} title={item.steerSent[s.key] ? '已发送' : `发送 ${s.label}`}>
                      <Button
                        size="small"
                        icon={item.steerSent[s.key] ? <span style={{ fontSize: 10 }}>✓</span> : <RocketOutlined />}
                        onClick={() => !item.steerSent[s.key] && sendSteerStep(item, s.key)}
                        disabled={item.steerSent[s.key] || sending}
                        style={{
                          fontSize: 10, height: 22, borderRadius: 4, padding: '0 6px',
                          color: item.steerSent[s.key] ? 'var(--text-muted)' : s.color,
                          borderColor: item.steerSent[s.key] ? 'var(--border)' : s.color,
                          opacity: item.steerSent[s.key] ? 0.5 : 1,
                        }}
                      >
                        {s.desc}
                      </Button>
                    </Tooltip>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Input area */}
      <div style={{ padding: '8px 24px 10px', borderTop: '1px solid var(--border)', background: 'var(--bg-root)' }}>
        <div style={{ maxWidth: 720, margin: '0 auto' }}>
          <div style={{
            display: 'flex', alignItems: 'flex-end', gap: 8,
            background: 'var(--bg-panel)', border: '1px solid var(--border)',
            borderRadius: 10, padding: '6px 8px',
          }}>
            <Dropdown
              menu={{
                items: [
                  { key: 'file', label: '添加文件', icon: <FileTextOutlined />,
                    onClick: () => {
                      const input = document.createElement('input');
                      input.type = 'file'; input.multiple = true;
                      input.onchange = (e: any) => {
                        const files = Array.from(e.target.files || []) as File[];
                        setInput(prev => prev + '\n' + files.map(f => f.name).join(', '));
                      };
                      input.click();
                    }
                  },
                  { key: 'divider', type: 'divider' },
                  { key: 'spec', label: '/spec — 写规格', icon: <FileTextOutlined />,
                    onClick: () => setInput(prev => (prev ? prev + '\n' : '') + '/spec ') },
                  { key: 'steer', label: '/steer — 全流程', icon: <BulbOutlined />,
                    onClick: () => setInput(prev => (prev ? prev + '\n' : '') + '/steer ') },
                  { key: 'execute', label: '/execute — 执行任务', icon: <RocketOutlined />,
                    onClick: () => setInput(prev => (prev ? prev + '\n' : '') + '/execute 1') },
                ]
              }}
              trigger={['click']}
            >
              <Button type="text" size="small"
                icon={<span style={{ fontWeight: 700, fontSize: 16 }}>+</span>}
                style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
            </Dropdown>
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
          {/* Bottom info row — model + access */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 6, padding: '0 4px' }}>
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
            <div style={{ flex: 1 }} />
            <Text type="secondary" style={{ fontSize: 10, color: 'var(--text-muted)' }}>
              {activeAgentProfile?.model?.model || 'deepseek-v4-pro'} · 完全访问
            </Text>
          </div>
        </div>
      </div>

      {/* Agent Floating Ball — draggable, top-left default */}
      <Popover
        open={ballOpen}
        onOpenChange={setBallOpen}
        trigger="click"
        placement="rightBottom"
        content={agentListContent}
        overlayStyle={{ maxWidth: 240 }}
      >
        <DraggableBall agentAvatar={agentAvatar} onBallClick={() => setBallOpen(v => !v)} />
      </Popover>

      {/* Sandbox Approval Modal */}
      <Modal
        title="🛡️ 工具调用确认"
        open={!!pendingApproval}
        onCancel={() => handleApprove(false)}
        footer={null}
        width={420}
        centered
        closable
      >
        {pendingApproval && (
          <Space direction="vertical" style={{ width: '100%' }} size={12}>
            <Result
              status="warning"
              title="需要你的确认"
              subTitle={
                <div style={{ textAlign: 'left' }}>
                  <p><strong>工具：</strong>{pendingApproval.tool}</p>
                  <p><strong>风险等级：</strong>
                    <Tag color={pendingApproval.risk === 'danger' ? 'red' : pendingApproval.risk === 'warning' ? 'orange' : 'green'}>
                      {pendingApproval.risk}
                    </Tag>
                  </p>
                  <p><strong>描述：</strong>{pendingApproval.description}</p>
                  <p style={{ fontSize: 11, color: '#999', wordBreak: 'break-all' }}>
                    <strong>参数：</strong>{pendingApproval.args}
                  </p>
                </div>
              }
            />
            <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
              <Button icon={<CloseCircleOutlined />} onClick={() => handleApprove(false)} danger>
                拒绝
              </Button>
              <Button type="primary" icon={<CheckCircleOutlined />} onClick={() => handleApprove(true)}>
                批准
              </Button>
            </div>
          </Space>
        )}
      </Modal>

      <style>{`
        @keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.4} }
        .chat-input-textarea { box-shadow: none !important; }
        .chat-input-textarea:focus { box-shadow: none !important; }
      `}</style>
    </div>
  );
}

function DraggableBall({ agentAvatar, onBallClick }: { agentAvatar: string; onBallClick: () => void }) {
  const [pos, setPos] = useState({ x: 20, y: 20 });
  const [dragging, setDragging] = useState(false);
  const offsetRef = useRef({ x: 0, y: 0 });
  const movedRef = useRef(false);
  const ballRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!dragging) return;
    const onMove = (e: MouseEvent) => {
      const dx = e.clientX - offsetRef.current.x;
      const dy = e.clientY - offsetRef.current.y;
      if (Math.abs(dx) > 3 || Math.abs(dy) > 3) movedRef.current = true;
      setPos({ x: dx, y: dy });
    };
    const onUp = () => setDragging(false);
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
    return () => { window.removeEventListener('mousemove', onMove); window.removeEventListener('mouseup', onUp); };
  }, [dragging]);

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault();
    movedRef.current = false;
    offsetRef.current = { x: e.clientX - pos.x, y: e.clientY - pos.y };
    setDragging(true);
  };

  const handleClick = () => {
    if (!movedRef.current) onBallClick();
  };

  return (
    <div
      ref={ballRef}
      onMouseDown={handleMouseDown}
      onClick={handleClick}
      style={{
        position: 'absolute',
        left: pos.x,
        top: pos.y,
        width: 44,
        height: 44,
        borderRadius: 22,
        background: 'linear-gradient(135deg, #5e6ad2, #7c5cfc)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontSize: 20,
        cursor: dragging ? 'grabbing' : 'grab',
        boxShadow: '0 4px 16px rgba(94,106,210,0.35)',
        transition: dragging ? 'none' : 'transform 0.15s, box-shadow 0.15s',
        userSelect: 'none',
        zIndex: 10,
      }}
      onMouseEnter={e => {
        if (dragging) return;
        e.currentTarget.style.transform = 'scale(1.08)';
        e.currentTarget.style.boxShadow = '0 6px 20px rgba(94,106,210,0.45)';
      }}
      onMouseLeave={e => {
        if (dragging) return;
        e.currentTarget.style.transform = 'scale(1)';
        e.currentTarget.style.boxShadow = '0 4px 16px rgba(94,106,210,0.35)';
      }}
    >
      {agentAvatar}
    </div>
  );
}

function Markdown({ text }: { text: string }) {
  const blocks = useMemo(() => {
    const parts: { type: 'text' | 'code' | 'image'; content: string; lang?: string; alt?: string }[] = [];
    // Extract code blocks first
    const codeRe = /```(\w*)\n([\s\S]*?)```/g;
    let cleaned = text;
    let m: RegExpExecArray | null;
    while ((m = codeRe.exec(text)) !== null) {
      cleaned = cleaned.replace(m[0], `\x00CODE\x00`);
    }
    // Extract markdown images from remaining text
    const imgRe = /!\[([^\]]*)\]\(([^)]+)\)/g;
    let last = 0;
    let imgMatch: RegExpExecArray | null;
    let codeIdx = 0;
    while ((imgMatch = imgRe.exec(cleaned)) !== null) {
      if (imgMatch.index > last) parts.push({ type: 'text', content: cleaned.slice(last, imgMatch.index) });
      parts.push({ type: 'image', content: imgMatch[2], alt: imgMatch[1] });
      last = imgMatch.index + imgMatch[0].length;
    }
    if (last < cleaned.length) {
      // Restore code blocks in remaining text
      let rest = cleaned.slice(last);
      codeRe.lastIndex = 0;
      const codeBlocks: string[] = [];
      while ((m = codeRe.exec(text)) !== null) codeBlocks.push(`\`\`\`${m[1]}\n${m[2]}\`\`\``);
      for (const cb of codeBlocks) rest = rest.replace('\x00CODE\x00', cb);
      if (rest.trim()) parts.push({ type: 'text', content: rest });
    }
    return parts;
  }, [text]);

  function renderText(plain: string): React.ReactNode[] {
    const pathRe = /([A-Z]:\\[\w\-\\]+\.\w+)/g;
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
    {blocks.map((b, i) => {
      if (b.type === 'code') return (
        <pre key={i} style={{
          background: 'var(--bg-panel)', border: '1px solid var(--border)', borderRadius: 8,
          padding: '10px 14px', margin: '6px 0', overflow: 'auto', fontSize: 12,
          fontFamily: "'JetBrains Mono', monospace",
        }}>
          {b.lang && <div style={{ fontSize: 10, color: 'var(--text-muted)', marginBottom: 4 }}>{b.lang}</div>}
          <code>{b.content}</code>
        </pre>
      );
      if (b.type === 'image') return (
        <div key={i} style={{ margin: '8px 0' }}>
          <img
            src={b.content}
            alt={b.alt || 'Generated image'}
            style={{ maxWidth: '100%', maxHeight: 512, borderRadius: 8, border: '1px solid var(--border)' }}
            loading="lazy"
          />
          {b.alt && <div style={{ fontSize: 10, color: 'var(--text-muted)', marginTop: 4, textAlign: 'center' }}>{b.alt}</div>}
        </div>
      );
      return <span key={i}>{renderText(b.content)}</span>;
    })}
  </>;
}
