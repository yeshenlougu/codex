import { useState, useMemo } from 'react';
import { Card, Input, Button, Typography, Tag, Row, Col, Tabs, Space, Empty, message } from 'antd';
import {
  SearchOutlined, DownloadOutlined, CheckCircleOutlined, AppstoreOutlined,
  ThunderboltOutlined, ChromeOutlined, WindowsOutlined, EyeOutlined,
  CodeOutlined, MoreOutlined, SettingOutlined, StarOutlined, PlusOutlined,
} from '@ant-design/icons';
import { installPlugin, uninstallPlugin, listPlugins } from '../lib/api';

const { Text, Title } = Typography;

interface PluginItem {
  key: string;
  name: string;
  description: string;
  icon: string; // emoji or component name
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

const DEVELOPMENT: PluginItem[] = [
  { key: 'vscode', name: 'VS Code', description: '与 VS Code 编辑器深度集成', icon: '💻', category: 'development', installed: false },
  { key: 'terminal-plugin', name: 'Terminal', description: '执行终端命令并获取输出', icon: '⚡', category: 'development', installed: true },
  { key: 'shell', name: 'Shell', description: '运行 Shell 脚本并管理进程', icon: '🐚', category: 'development', source: 'openai-api-curated', installed: false },
];

const COMMUNICATION: PluginItem[] = [
  { key: 'feishu', name: '飞书', description: '读取飞书文档、发送消息', icon: '🐦', category: 'communication', installed: false },
  { key: 'dingtalk', name: '钉钉', description: '钉钉消息与机器人集成', icon: '📱', category: 'communication', installed: false },
  { key: 'wechat', name: '微信', description: '企业微信消息与通知', icon: '💚', category: 'communication', installed: false },
];

const ALL_PLUGINS: PluginItem[] = [...FEATURED, ...PRODUCTIVITY, ...DEVELOPMENT, ...COMMUNICATION];

const CATEGORIES = [
  { key: 'featured', label: '精选', icon: <StarOutlined /> },
  { key: 'productivity', label: '生产力', icon: <ThunderboltOutlined /> },
  { key: 'development', label: '开发', icon: <CodeOutlined /> },
  { key: 'communication', label: '通讯', icon: <ChromeOutlined /> },
];

export default function PluginsPage() {
  const [tab, setTab] = useState<'plugins' | 'skills'>('plugins');
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
          name: plugin.key,
          description: plugin.description,
          command: plugin.key,
          args: [],
          schema: { type: 'object', properties: {} },
        });
        message.success(`${plugin.name} 已安装`);
      }
      setInstalled(prev => {
        const next = new Set(prev);
        if (next.has(plugin.key)) next.delete(plugin.key);
        else next.add(plugin.key);
        return next;
      });
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

  return (
    <div style={{ height: '100%', overflowY: 'auto', background: 'var(--bg-root)' }}>
      <div style={{ maxWidth: 780, margin: '0 auto', padding: '24px' }}>
        {/* Tabs */}
        <Tabs
          activeKey={tab}
          onChange={k => setTab(k as 'plugins' | 'skills')}
          items={[
            { key: 'plugins', label: '插件' },
            { key: 'skills', label: '技能' },
          ]}
          tabBarStyle={{ marginBottom: 8 }}
        />

        {tab === 'plugins' ? (
          <>
            {/* Title */}
            <Title level={3} style={{ margin: 0, fontSize: 20, color: 'var(--text-primary)' }}>
              插件
            </Title>
            <Text type="secondary" style={{ fontSize: 13, marginBottom: 16, display: 'block' }}>
              在你常用的工具中与 Codex 协作
            </Text>

            {/* Search */}
            <Input
              prefix={<SearchOutlined style={{ color: 'var(--text-muted)' }} />}
              placeholder="搜索插件"
              value={search}
              onChange={e => setSearch(e.target.value)}
              allowClear
              style={{ marginBottom: 20, borderRadius: 8 }}
              size="large"
            />

            {/* Installed */}
            {installedPlugins.length > 0 && (
              <div style={{ marginBottom: 20 }}>
                <Text strong style={{ fontSize: 11, textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-muted)', marginBottom: 8, display: 'block' }}>
                  已安装
                </Text>
                <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
                  {installedPlugins.map(p => (
                    <Card
                      key={p.key}
                      size="small"
                      style={{ width: 180, cursor: 'pointer' }}
                      bodyStyle={{ padding: '12px', display: 'flex', flexDirection: 'column', gap: 6 }}
                    >
                      <Text style={{ fontSize: 20 }}>{p.icon}</Text>
                      <Text strong style={{ fontSize: 12 }}>{p.name}</Text>
                      <Text type="secondary" style={{ fontSize: 10, lineHeight: 1.4 }}>{p.description}</Text>
                    </Card>
                  ))}
                </div>
              </div>
            )}

            {/* Category tabs */}
            <div style={{ display: 'flex', gap: 6, marginBottom: 16, flexWrap: 'wrap' }}>
              {CATEGORIES.map(cat => (
                <Button
                  key={cat.key}
                  size="small"
                  type={activeCategory === cat.key ? 'primary' : 'default'}
                  icon={cat.icon}
                  onClick={() => setActiveCategory(cat.key)}
                  style={{ borderRadius: 6 }}
                >
                  {cat.label}
                </Button>
              ))}
              <Button
                size="small"
                type={activeCategory === 'all' ? 'primary' : 'default'}
                onClick={() => setActiveCategory('all')}
                style={{ borderRadius: 6 }}
              >
                全部
              </Button>
            </div>

            {/* Plugin grid */}
            {filteredPlugins.length === 0 ? (
              <Empty description={<Text type="secondary">未找到匹配的插件</Text>} style={{ padding: '40px 0' }} />
            ) : (
              <Row gutter={[12, 12]}>
                {filteredPlugins.map(p => (
                  <Col xs={24} sm={12} key={p.key}>
                    <Card
                      size="small"
                      hoverable
                      bodyStyle={{ padding: '14px', display: 'flex', gap: 12, alignItems: 'flex-start' }}
                    >
                      <Text style={{ fontSize: 24, flexShrink: 0, marginTop: 2 }}>{p.icon}</Text>
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 2 }}>
                          <Text strong style={{ fontSize: 13 }}>{p.name}</Text>
                          {p.source && (
                            <Text type="secondary" style={{ fontSize: 9, color: 'var(--text-muted)' }}>{p.source}</Text>
                          )}
                        </div>
                        <Text type="secondary" style={{ fontSize: 11, lineHeight: 1.4, display: 'block', marginBottom: 8 }}>
                          {p.description}
                        </Text>
                        <Button
                          size="small"
                          type={installed.has(p.key) ? 'default' : 'primary'}
                          icon={installed.has(p.key) ? <CheckCircleOutlined /> : <DownloadOutlined />}
                          onClick={(e) => { e.stopPropagation(); toggleInstall(p); }}
                          style={{ borderRadius: 6, fontSize: 11 }}
                        >
                          {installed.has(p.key) ? '已安装' : '安装'}
                        </Button>
                      </div>
                    </Card>
                  </Col>
                ))}
              </Row>
            )}
          </>
        ) : (
          /* Skills Tab */
          <>
            <Title level={3} style={{ margin: 0, fontSize: 20, color: 'var(--text-primary)' }}>
              技能
            </Title>
            <Text type="secondary" style={{ fontSize: 13, marginBottom: 16, display: 'block' }}>
              管理和发现你的 Codex 技能 (SKILL.md)
            </Text>

            <Input
              prefix={<SearchOutlined style={{ color: 'var(--text-muted)' }} />}
              placeholder="搜索技能..."
              allowClear
              style={{ marginBottom: 20, borderRadius: 8 }}
              size="large"
            />

            <Row gutter={[12, 12]}>
              {[
                { name: 'code-review', desc: '代码审查与建议改进', icon: '🔍' },
                { name: 'refactor', desc: '重构代码模式与优化', icon: '♻️' },
                { name: 'test-gen', desc: '自动生成单元测试', icon: '🧪' },
                { name: 'docs-gen', desc: '自动生成 API 文档', icon: '📖' },
                { name: 'db-migrate', desc: '数据库迁移与 Schema 管理', icon: '🗄️' },
                { name: 'deploy', desc: '自动化部署与 CI/CD', icon: '🚀' },
              ].map(skill => (
                <Col xs={24} sm={12} key={skill.name}>
                  <Card size="small" hoverable bodyStyle={{ padding: '14px', display: 'flex', gap: 12, alignItems: 'center' }}>
                    <Text style={{ fontSize: 22, flexShrink: 0 }}>{skill.icon}</Text>
                    <div style={{ flex: 1 }}>
                      <Text strong style={{ fontSize: 13 }}>{skill.name}</Text>
                      <Text type="secondary" style={{ fontSize: 11, display: 'block', lineHeight: 1.4 }}>{skill.desc}</Text>
                    </div>
                    <Button type="text" size="small" icon={<SettingOutlined />} />
                  </Card>
                </Col>
              ))}
            </Row>

            <div style={{ marginTop: 20, padding: 16, background: 'var(--bg-panel)', borderRadius: 8, border: '1px solid var(--border)' }}>
              <Text style={{ display: 'block', marginBottom: 8 }}>
                💡 <Text strong>创建自定义技能</Text> — 将常用工作流程保存为 SKILL.md
              </Text>
              <Button type="primary" size="small" icon={<PlusOutlined />}>
                新建技能
              </Button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
