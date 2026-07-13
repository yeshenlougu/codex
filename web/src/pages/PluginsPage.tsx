import { useState, useMemo, useEffect } from 'react';
import { Card, Input, Button, Typography, Tag, Row, Col, Tabs, Space, Empty, message, Spin, Modal, Form, Select, Badge, Descriptions } from 'antd';
import {
  SearchOutlined, DownloadOutlined, CheckCircleOutlined, AppstoreOutlined,
  ThunderboltOutlined, ChromeOutlined, WindowsOutlined, EyeOutlined,
  CodeOutlined, MoreOutlined, SettingOutlined, StarOutlined, PlusOutlined,
  DeleteOutlined, ReloadOutlined, LinkOutlined, ApiOutlined, ToolOutlined,
  CheckOutlined, ExclamationCircleOutlined, DisconnectOutlined,
} from '@ant-design/icons';
import {
  installPlugin, uninstallPlugin, listPlugins, listSkills,
  listInstalledSkills, discoverSkills, installSkill, uninstallSkill, checkSkillUpdates,
  listSkillRepos, addSkillRepo, removeSkillRepo,
  listMCPServers, createMCPServer, deleteMCPServer, restartMCPServer, getMCPPresets,
} from '../lib/api';
import type { SkillInfo, InstalledSkill, DiscoverSkill, SkillRepoItem } from '../lib/api';
import type { MCPServer, MCPPreset } from '../lib/api';

const { Text, Title } = Typography;

// ===================== Plugin Types (hardcoded catalog) =====================

interface PluginItem {
  key: string;
  name: string;
  description: string;
  icon: string;
  category: 'featured' | 'productivity' | 'development' | 'communication';
  source?: string;
  installed: boolean;
}

const FEATURED: PluginItem[] = [
  { key: 'computer-use', name: 'Computer Use', description: '让 Codex 控制你的桌面应用', icon: '🖥️', category: 'featured', installed: true },
  { key: 'chrome', name: 'Chrome', description: '让 Codex 控制浏览器', icon: '🌐', category: 'featured', installed: false },
];

const PRODUCTIVITY: PluginItem[] = [
  { key: 'chrome-prod', name: 'Chrome', description: '让 Codex 控制浏览器', icon: '🌐', category: 'productivity', installed: false },
  { key: 'computer-use-prod', name: 'Computer Use', description: '让 Codex 控制你的桌面应用', icon: '🖥️', category: 'productivity', installed: true },
  { key: 'visualize', name: 'Visualize', description: '将想法和数据转化为交互式可视化', icon: '✨', category: 'featured', installed: true },
  { key: 'linear', name: 'Linear', description: '查找和引用 Linear issues 与项目', icon: '📐', category: 'productivity', source: 'openai-api-curated', installed: false },
  { key: 'notion', name: 'Notion', description: '搜索和读取 Notion 页面与数据库', icon: '📝', category: 'productivity', source: 'openai-api-curated', installed: false },
  { key: 'github', name: 'GitHub', description: '管理 Issues、PR 和代码审查', icon: '🐙', category: 'development', installed: false },
  { key: 'slack', name: 'Slack', description: '在 Slack 中发送消息与搜索频道', icon: '💬', category: 'communication', installed: false },
];

const ALL_PLUGINS: PluginItem[] = [...FEATURED, ...PRODUCTIVITY];

const CATEGORIES = [
  { key: 'featured', label: '精选', icon: <StarOutlined /> },
  { key: 'productivity', label: '生产力', icon: <ThunderboltOutlined /> },
  { key: 'development', label: '开发', icon: <CodeOutlined /> },
  { key: 'communication', label: '通讯', icon: <ChromeOutlined /> },
];

export default function PluginsPage() {
  const [tab, setTab] = useState<'plugins' | 'skills' | 'mcp'>('plugins');
  return (
    <div style={{ height: '100%', overflowY: 'auto', background: 'var(--bg-root)' }}>
      <div style={{ maxWidth: 780, margin: '0 auto', padding: '24px' }}>
        <Tabs activeKey={tab} onChange={k => setTab(k as typeof tab)}
          items={[
            { key: 'plugins', label: '插件' },
            { key: 'skills', label: '技能' },
            { key: 'mcp', label: 'MCP' },
          ]}
          tabBarStyle={{ marginBottom: 8 }}
        />
        {tab === 'plugins' && <PluginsContent />}
        {tab === 'skills' && <SkillsContent />}
        {tab === 'mcp' && <MCPContent />}
      </div>
    </div>
  );
}

// ===================== Plugins Tab =====================

function PluginsContent() {
  const [search, setSearch] = useState('');
  const [activeCategory, setActiveCategory] = useState('featured');
  const [installed, setInstalled] = useState<Set<string>>(
    new Set(ALL_PLUGINS.filter(p => p.installed).map(p => p.key))
  );

  const toggleInstall = async (plugin: PluginItem) => {
    try {
      if (installed.has(plugin.key)) {
        await uninstallPlugin(plugin.key);
        message.success(`${plugin.name} 已卸载`);
      } else {
        await installPlugin({
          name: plugin.key, description: plugin.description,
          command: plugin.key, args: [], schema: { type: 'object', properties: {} },
        });
        message.success(`${plugin.name} 已安装`);
      }
      setInstalled(prev => { const n = new Set(prev); if (n.has(plugin.key)) n.delete(plugin.key); else n.add(plugin.key); return n; });
    } catch (e: any) { message.error(e.message); }
  };

  const filteredPlugins = useMemo(() => {
    const q = search.toLowerCase();
    return ALL_PLUGINS.filter(p => {
      const matchCat = activeCategory === 'all' || p.category === activeCategory;
      const matchSearch = !q || p.name.toLowerCase().includes(q) || p.description.toLowerCase().includes(q);
      return matchCat && matchSearch;
    });
  }, [search, activeCategory]);

  const installedPlugins = ALL_PLUGINS.filter(p => installed.has(p.key));

  return <>
    <Title level={3} style={{ margin: 0, fontSize: 20, color: 'var(--text-primary)' }}>插件</Title>
    <Text type="secondary" style={{ fontSize: 13, marginBottom: 16, display: 'block' }}>在你常用的工具中与 Codex 协作</Text>
    <Input prefix={<SearchOutlined style={{ color: 'var(--text-muted)' }} />} placeholder="搜索插件" value={search}
      onChange={e => setSearch(e.target.value)} allowClear style={{ marginBottom: 20, borderRadius: 8 }} size="large" />
    {installedPlugins.length > 0 && (
      <div style={{ marginBottom: 20 }}>
        <Text strong style={{ fontSize: 11, textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-muted)', marginBottom: 8, display: 'block' }}>已安装</Text>
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
          {installedPlugins.map(p => (
            <Card key={p.key} size="small" style={{ width: 180, cursor: 'pointer' }} bodyStyle={{ padding: '12px' }}>
              <Text style={{ fontSize: 20 }}>{p.icon}</Text>
              <Text strong style={{ fontSize: 12, display: 'block' }}>{p.name}</Text>
              <Text type="secondary" style={{ fontSize: 10 }}>{p.description}</Text>
            </Card>
          ))}
        </div>
      </div>
    )}
    <div style={{ display: 'flex', gap: 6, marginBottom: 16, flexWrap: 'wrap' }}>
      {CATEGORIES.map(cat => (
        <Button key={cat.key} size="small" type={activeCategory === cat.key ? 'primary' : 'default'} icon={cat.icon}
          onClick={() => setActiveCategory(cat.key)} style={{ borderRadius: 6 }}>{cat.label}</Button>
      ))}
      <Button size="small" type={activeCategory === 'all' ? 'primary' : 'default'}
        onClick={() => setActiveCategory('all')} style={{ borderRadius: 6 }}>全部</Button>
    </div>
    {filteredPlugins.length === 0 ? (
      <Empty description={<Text type="secondary">未找到匹配的插件</Text>} style={{ padding: '40px 0' }} />
    ) : (
      <Row gutter={[12, 12]}>
        {filteredPlugins.map(p => (
          <Col xs={24} sm={12} key={p.key}>
            <Card size="small" hoverable bodyStyle={{ padding: '14px', display: 'flex', gap: 12, alignItems: 'flex-start' }}>
              <Text style={{ fontSize: 24, flexShrink: 0, marginTop: 2 }}>{p.icon}</Text>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 2 }}>
                  <Text strong style={{ fontSize: 13 }}>{p.name}</Text>
                  {p.source && <Text type="secondary" style={{ fontSize: 9 }}>{p.source}</Text>}
                </div>
                <Text type="secondary" style={{ fontSize: 11, lineHeight: 1.4, display: 'block', marginBottom: 8 }}>{p.description}</Text>
                <Button size="small" type={installed.has(p.key) ? 'default' : 'primary'}
                  icon={installed.has(p.key) ? <CheckCircleOutlined /> : <DownloadOutlined />}
                  onClick={(e) => { e.stopPropagation(); toggleInstall(p); }} style={{ borderRadius: 6, fontSize: 11 }}>
                  {installed.has(p.key) ? '已安装' : '安装'}
                </Button>
              </div>
            </Card>
          </Col>
        ))}
      </Row>
    )}
  </>;
}

// ===================== Skills Tab =====================

function SkillsContent() {
  const [subtab, setSubtab] = useState<'installed' | 'discover' | 'repos'>('installed');
  const [installed, setInstalled] = useState<InstalledSkill[]>([]);
  const [discover, setDiscover] = useState<DiscoverSkill[]>([]);
  const [repos, setRepos] = useState<SkillRepoItem[]>([]);
  const [outdated, setOutdated] = useState<InstalledSkill[]>([]);
  const [loading, setLoading] = useState(false);
  const [repoModal, setRepoModal] = useState(false);
  const [repoOwner, setRepoOwner] = useState('');
  const [repoName, setRepoName] = useState('');
  const [repoBranch, setRepoBranch] = useState('main');

  const loadInstalled = () => { setLoading(true); listInstalledSkills().then(r => setInstalled(r.skills || [])).catch(() => {}).finally(() => setLoading(false)); };
  const loadDiscover = () => { setLoading(true); discoverSkills().then(r => setDiscover(r.skills || [])).catch(() => {}).finally(() => setLoading(false)); };
  const loadRepos = () => { listSkillRepos().then(r => setRepos(r.repos || [])).catch(() => {}); };
  const loadUpdates = () => { checkSkillUpdates().then(r => setOutdated(r.outdated || [])).catch(() => {}); };

  useEffect(() => { loadInstalled(); loadRepos(); loadUpdates(); }, []);
  useEffect(() => { if (subtab === 'discover') loadDiscover(); }, [subtab]);

  const doInstall = async (s: DiscoverSkill) => {
    try { await installSkill(s.key, s.repo_owner, s.repo_name, s.repo_branch, s.directory); message.success(`${s.name} 安装成功`); loadInstalled(); loadDiscover(); } catch (e: any) { message.error(e.message); }
  };
  const doUninstall = async (id: string) => {
    try { await uninstallSkill(id); message.success('已卸载'); loadInstalled(); } catch (e: any) { message.error(e.message); }
  };
  const doAddRepo = async () => {
    if (!repoOwner || !repoName) return;
    try { await addSkillRepo(repoOwner, repoName, repoBranch); message.success('仓库已添加'); setRepoModal(false); loadRepos(); } catch (e: any) { message.error(e.message); }
  };
  const doRemoveRepo = async (owner: string, name: string) => {
    try { await removeSkillRepo(owner, name); message.success('仓库已移除'); loadRepos(); } catch (e: any) { message.error(e.message); }
  };

  return <>
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
      <Title level={3} style={{ margin: 0, fontSize: 20, color: 'var(--text-primary)' }}>技能</Title>
      <Space>
        {outdated.length > 0 && <Badge count={outdated.length}><Button size="small" icon={<ExclamationCircleOutlined />}>可更新</Button></Badge>}
        <Button size="small" icon={<ReloadOutlined />} onClick={() => { loadInstalled(); loadUpdates(); }}>刷新</Button>
      </Space>
    </div>
    <Text type="secondary" style={{ fontSize: 13, marginBottom: 16, display: 'block' }}>管理和发现你的 Codex Skills (SKILL.md)</Text>

    <Tabs activeKey={subtab} onChange={k => setSubtab(k as typeof subtab)} size="small"
      items={[
        { key: 'installed', label: `已安装 (${installed.length})` },
        { key: 'discover', label: '发现' },
        { key: 'repos', label: '仓库' },
      ]}
      tabBarStyle={{ marginBottom: 12 }}
    />

    {subtab === 'installed' && (
      loading ? <Spin style={{ display: 'block', textAlign: 'center', padding: 24 }} /> :
      installed.length === 0 ? <Empty description="暂无已安装的技能" /> :
      <Row gutter={[12, 12]}>
        {installed.map(s => (
          <Col xs={24} sm={12} key={s.id}>
            <Card size="small" hoverable bodyStyle={{ padding: '12px', display: 'flex', gap: 10, alignItems: 'center' }}
              actions={[<Button type="text" size="small" danger icon={<DeleteOutlined />} onClick={() => doUninstall(s.id)} key="del">卸载</Button>]}>
              <Text style={{ fontSize: 20 }}>📄</Text>
              <div style={{ flex: 1 }}>
                <Text strong style={{ fontSize: 12 }}>{s.name}</Text>
                <Text type="secondary" style={{ fontSize: 10, display: 'block' }}>{s.description || '无描述'}</Text>
                <Text type="secondary" style={{ fontSize: 9, display: 'block' }}>
                  {s.repo_owner}/{s.repo_name} · {s.directory}
                </Text>
              </div>
            </Card>
          </Col>
        ))}
      </Row>
    )}

    {subtab === 'discover' && (
      loading ? <Spin style={{ display: 'block', textAlign: 'center', padding: 24 }} /> :
      discover.length === 0 ? <Empty description="未发现可安装技能 — 检查仓库配置"> </Empty> :
      <Row gutter={[12, 12]}>
        {discover.map(s => (
          <Col xs={24} sm={12} key={s.key}>
            <Card size="small" hoverable bodyStyle={{ padding: '12px', display: 'flex', gap: 10, alignItems: 'center' }}>
              <Text style={{ fontSize: 20 }}>📦</Text>
              <div style={{ flex: 1 }}>
                <Text strong style={{ fontSize: 12 }}>{s.name}</Text>
                <Text type="secondary" style={{ fontSize: 10, display: 'block' }}>{s.description || '无描述'}</Text>
                <Text type="secondary" style={{ fontSize: 9 }}>{s.repo_owner}/{s.repo_name} · {s.directory}</Text>
              </div>
              <Button size="small" type={s.installed ? 'default' : 'primary'} disabled={s.installed}
                icon={s.installed ? <CheckOutlined /> : <DownloadOutlined />}
                onClick={() => doInstall(s)} style={{ borderRadius: 6 }}>
                {s.installed ? '已安装' : '安装'}
              </Button>
            </Card>
          </Col>
        ))}
      </Row>
    )}

    {subtab === 'repos' && <>
      <Button type="dashed" size="small" icon={<PlusOutlined />} onClick={() => setRepoModal(true)} style={{ marginBottom: 12 }}>添加仓库</Button>
      {repos.length === 0 ? <Empty description="无配置的仓库" /> :
      repos.map(r => (
        <Card key={`${r.owner}/${r.name}`} size="small" style={{ marginBottom: 8 }} bodyStyle={{ padding: '10px 14px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Space>
            <LinkOutlined />
            <Text strong style={{ fontSize: 12 }}>{r.owner}/{r.name}</Text>
            <Tag color="blue" style={{ fontSize: 9 }}>{r.branch}</Tag>
            {!r.enabled && <Tag color="default" style={{ fontSize: 9 }}>已禁用</Tag>}
          </Space>
          <Button type="text" size="small" danger icon={<DeleteOutlined />} onClick={() => doRemoveRepo(r.owner, r.name)} />
        </Card>
      ))}
      <Modal title="添加 Skill 仓库" open={repoModal} onOk={doAddRepo} onCancel={() => setRepoModal(false)} okText="添加">
        <Form layout="vertical" size="small">
          <Form.Item label="Owner"><Input value={repoOwner} onChange={e => setRepoOwner(e.target.value)} placeholder="例如 anthropics" /></Form.Item>
          <Form.Item label="Repo"><Input value={repoName} onChange={e => setRepoName(e.target.value)} placeholder="例如 skills" /></Form.Item>
          <Form.Item label="Branch"><Input value={repoBranch} onChange={e => setRepoBranch(e.target.value)} /></Form.Item>
        </Form>
      </Modal>
    </>}
  </>;
}

// ===================== MCP Tab =====================

function MCPContent() {
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [presets, setPresets] = useState<MCPPreset[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form] = Form.useForm();

  const load = () => { setLoading(true); listMCPServers().then(r => setServers(r.servers || [])).catch(() => {}).finally(() => setLoading(false)); };
  useEffect(() => { load(); getMCPPresets().then(r => setPresets(r.presets || [])).catch(() => {}); }, []);

  const handleCreate = async (values: any) => {
    try {
      await createMCPServer(values);
      message.success('MCP 服务器已添加');
      setModalOpen(false); form.resetFields(); load();
    } catch (e: any) { message.error(e.message); }
  };

  const handleDelete = async (id: string) => {
    try { await deleteMCPServer(id); message.success('已删除'); load(); } catch (e: any) { message.error(e.message); }
  };

  const handleRestart = async (id: string) => {
    try { await restartMCPServer(id); message.success('已重启'); load(); } catch (e: any) { message.error(e.message); }
  };

  const applyPreset = (p: MCPPreset) => {
    form.setFieldsValue({ name: p.name, description: p.description, command: p.command, args: p.args, env: p.env });
    setModalOpen(true);
  };

  return <>
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
      <Title level={3} style={{ margin: 0, fontSize: 20, color: 'var(--text-primary)' }}>MCP 服务器</Title>
      <Space>
        <Button size="small" icon={<ReloadOutlined />} onClick={load}>刷新</Button>
        <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => { setEditingId(null); form.resetFields(); setModalOpen(true); }}>添加</Button>
      </Space>
    </div>
    <Text type="secondary" style={{ fontSize: 13, marginBottom: 16, display: 'block' }}>管理 MCP (Model Context Protocol) 服务器 — 为 Agent 扩展外部工具</Text>

    {/* Presets */}
    <div style={{ marginBottom: 16 }}>
      <Text strong style={{ fontSize: 11, color: 'var(--text-muted)', display: 'block', marginBottom: 8 }}>快速模板</Text>
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
        {presets.map(p => (
          <Card key={p.name} size="small" hoverable style={{ width: 200, cursor: 'pointer' }} bodyStyle={{ padding: '10px' }}
            onClick={() => applyPreset(p)}>
            <Text strong style={{ fontSize: 11, display: 'block' }}><ApiOutlined /> {p.name}</Text>
            <Text type="secondary" style={{ fontSize: 10 }}>{p.description}</Text>
            <div style={{ marginTop: 4 }}><Tag style={{ fontSize: 9 }}>{p.command} {p.args.slice(0, 2).join(' ')}</Tag></div>
          </Card>
        ))}
      </div>
    </div>

    {/* Server list */}
    {loading ? <Spin style={{ display: 'block', textAlign: 'center', padding: 24 }} /> :
     servers.length === 0 ? <Empty description="暂无 MCP 服务器 — 点击添加或选择快速模板" /> :
     servers.map(srv => (
      <Card key={srv.id} size="small" style={{ marginBottom: 8 }} bodyStyle={{ padding: '12px 16px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <div style={{ flex: 1 }}>
            <Space>
              <Text strong style={{ fontSize: 13 }}>{srv.name}</Text>
              <Badge status={srv.status === 'connected' ? 'success' : srv.status === 'error' ? 'error' : 'default'}
                text={srv.status === 'connected' ? '已连接' : srv.status === 'error' ? '错误' : '未连接'} />
              {srv.status === 'connected' && <Tag color="blue" style={{ fontSize: 9 }}>{srv.tool_count} 个工具</Tag>}
            </Space>
            {srv.description && <Text type="secondary" style={{ fontSize: 10, display: 'block' }}>{srv.description}</Text>}
            <Text type="secondary" style={{ fontSize: 9, fontFamily: 'monospace', display: 'block' }}>
              {srv.command} {srv.args.join(' ')}
            </Text>
            {srv.error && <Text type="danger" style={{ fontSize: 9, display: 'block' }}>{srv.error}</Text>}
          </div>
          <Space>
            <Button size="small" icon={<ReloadOutlined />} onClick={() => handleRestart(srv.id)}>重启</Button>
            <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(srv.id)} />
          </Space>
        </div>
      </Card>
    ))}

    {/* Add/Edit Modal */}
    <Modal title="添加 MCP 服务器" open={modalOpen} onCancel={() => setModalOpen(false)} footer={null} width={520}>
      <Form form={form} layout="vertical" size="small" onFinish={handleCreate} style={{ marginTop: 16 }}>
        <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input placeholder="例如 Filesystem" /></Form.Item>
        <Form.Item name="description" label="描述"><Input placeholder="简要描述此服务器的功能" /></Form.Item>
        <Form.Item name="command" label="命令" rules={[{ required: true }]}><Input placeholder="例如 npx 或 uvx" /></Form.Item>
        <Form.Item name="args" label="参数">
          <Select mode="tags" placeholder="输入参数后按回车" style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="enabled" label="启用" initialValue={true}>
          <Select options={[{ value: true, label: '是' }, { value: false, label: '否' }]} />
        </Form.Item>
        <Button type="primary" htmlType="submit">添加</Button>
      </Form>
    </Modal>
  </>;
}
