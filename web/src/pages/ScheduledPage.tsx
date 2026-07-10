import { useState } from 'react';
import { Card, Button, Typography, Row, Col, Tag, Empty, Modal, Select, TimePicker, Input, message } from 'antd';
import {
  CalendarOutlined, MailOutlined, FlagOutlined, PlusOutlined,
  ClockCircleOutlined, BellOutlined, LineChartOutlined, ArrowRightOutlined, BulbOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';

const { Text, Title } = Typography;

interface ScheduleTemplate {
  icon: React.ReactNode;
  title: string;
  schedule: string;
  description: string;
  category: 'daily' | 'weekly' | 'monitor';
}

const templates: ScheduleTemplate[] = [
  {
    icon: <CalendarOutlined style={{ fontSize: 22, color: '#5e6ad2' }} />,
    title: '每日简报',
    schedule: '工作日 8:00',
    description: '以日历、未读电子邮件和优先事项摘要开启每个工作日',
    category: 'daily',
  },
  {
    icon: <LineChartOutlined style={{ fontSize: 22, color: '#7c5cfc' }} />,
    title: '每周回顾',
    schedule: '星期五 16:00',
    description: '每周五将你最近的工作整理成简明的状态更新',
    category: 'weekly',
  },
  {
    icon: <BellOutlined style={{ fontSize: 22, color: '#27a644' }} />,
    title: '跟进监控',
    schedule: '工作日 9:00',
    description: '查看最近的电子邮箱和日历活动，并标记需要你关注的事项',
    category: 'monitor',
  },
  {
    icon: <MailOutlined style={{ fontSize: 22, color: '#d19a00' }} />,
    title: '邮件摘要',
    schedule: '每日 7:30',
    description: '汇总未读邮件并按优先级排序',
    category: 'daily',
  },
  {
    icon: <FlagOutlined style={{ fontSize: 22, color: '#e5484d' }} />,
    title: '任务检查',
    schedule: '每日 10:00',
    description: '检查待办任务进度并提醒即将到期的事项',
    category: 'daily',
  },
];

const categoryLabels: Record<ScheduleTemplate['category'], string> = {
  daily: '每日',
  weekly: '每周',
  monitor: '监控',
};

const categoryColors: Record<ScheduleTemplate['category'], string> = {
  daily: 'blue',
  weekly: 'purple',
  monitor: 'green',
};

export default function ScheduledPage() {
  const [createOpen, setCreateOpen] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState('');
  const [taskName, setTaskName] = useState('');

  const handleCreate = () => {
    if (!taskName.trim()) {
      message.warning('请输入任务名称');
      return;
    }
    message.success({ content: `已创建定时任务: ${taskName}`, duration: 2 });
    setCreateOpen(false);
    setTaskName('');
    setSelectedTemplate('');
  };

  return (
    <div style={{ height: '100%', overflowY: 'auto', background: 'var(--bg-root)' }}>
      <div style={{ maxWidth: 720, margin: '0 auto', padding: '32px 24px' }}>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
          <div>
            <Title level={3} style={{ margin: 0, fontSize: 20, color: 'var(--text-primary)' }}>
              已安排的任务
            </Title>
            <Text type="secondary" style={{ fontSize: 13, marginTop: 4, display: 'block' }}>
              让 Codex 安排任务、设置提醒或监测更新
            </Text>
          </div>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setCreateOpen(true)}
            style={{ borderRadius: 6 }}
          >
            创建
          </Button>
        </div>

        {/* Suggestions */}
        <div style={{ marginTop: 24 }}>
          <Text type="secondary" style={{ fontSize: 11, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 12, display: 'block' }}>
            建议
          </Text>
          <Row gutter={[12, 12]}>
            {templates.map((tpl, i) => (
              <Col span={24} key={i}>
                <Card
                  size="small"
                  hoverable
                  onClick={() => {
                    setSelectedTemplate(tpl.title);
                    setTaskName(tpl.title);
                    setCreateOpen(true);
                  }}
                  style={{ cursor: 'pointer' }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
                    <div style={{ flexShrink: 0 }}>
                      {tpl.icon}
                    </div>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                        <Text strong style={{ fontSize: 13 }}>{tpl.title}</Text>
                        <Tag color={categoryColors[tpl.category]} style={{ margin: 0, fontSize: 10, lineHeight: '18px' }}>
                          {categoryLabels[tpl.category]}
                        </Tag>
                      </div>
                      <Text type="secondary" style={{ fontSize: 11, display: 'block', marginBottom: 2 }}>
                        {tpl.description}
                      </Text>
                      <Text type="secondary" style={{ fontSize: 10, color: 'var(--text-muted)' }}>
                        <ClockCircleOutlined style={{ marginRight: 4 }} />{tpl.schedule}
                      </Text>
                    </div>
                    <ArrowRightOutlined style={{ color: 'var(--text-muted)', fontSize: 12, flexShrink: 0 }} />
                  </div>
                </Card>
              </Col>
            ))}
          </Row>
        </div>

        {/* Active schedules */}
        <div style={{ marginTop: 32 }}>
          <Text type="secondary" style={{ fontSize: 11, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 12, display: 'block' }}>
            活跃任务
          </Text>
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={<Text type="secondary" style={{ fontSize: 12 }}>暂无活跃的定时任务</Text>}
          >
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
              创建第一个定时任务
            </Button>
          </Empty>
        </div>
      </div>

      {/* Create Modal */}
      <Modal
        title="创建定时任务"
        open={createOpen}
        onCancel={() => { setCreateOpen(false); setTaskName(''); setSelectedTemplate(''); }}
        onOk={handleCreate}
        okText="创建"
        cancelText="取消"
        width={480}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16, paddingTop: 8 }}>
          <div>
            <Text style={{ fontSize: 12, fontWeight: 500, marginBottom: 6, display: 'block' }}>任务名称</Text>
            <Input
              placeholder="输入任务名称..."
              value={taskName}
              onChange={e => setTaskName(e.target.value)}
            />
          </div>

          {selectedTemplate && (
            <div style={{
              padding: '8px 12px', background: 'var(--bg-active)', borderRadius: 6,
              display: 'flex', alignItems: 'center', gap: 8, fontSize: 12,
            }}>
              <BulbOutlined style={{ color: '#5e6ad2' }} />
              <Text type="secondary">基于模板: <Text strong>{selectedTemplate}</Text></Text>
            </div>
          )}

          <div>
            <Text style={{ fontSize: 12, fontWeight: 500, marginBottom: 6, display: 'block' }}>频率</Text>
            <Select
              style={{ width: '100%' }}
              defaultValue="daily"
              options={[
                { value: 'daily', label: '每日' },
                { value: 'weekdays', label: '工作日' },
                { value: 'weekly', label: '每周' },
                { value: 'monthly', label: '每月' },
                { value: 'custom', label: '自定义 (Cron)' },
              ]}
            />
          </div>

          <div>
            <Text style={{ fontSize: 12, fontWeight: 500, marginBottom: 6, display: 'block' }}>时间</Text>
            <TimePicker format="HH:mm" defaultValue={dayjs('08:00', 'HH:mm')} style={{ width: '100%' }} />
          </div>
        </div>
      </Modal>
    </div>
  );
}
