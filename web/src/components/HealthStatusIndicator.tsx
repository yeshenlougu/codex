import { Tooltip, Badge } from 'antd';
import {
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  CloseCircleOutlined,
  QuestionCircleOutlined,
} from '@ant-design/icons';

type HealthStatus = 'healthy' | 'degraded' | 'unhealthy' | 'unknown';

const STATUS_CONFIG: Record<HealthStatus, { color: string; icon: React.ReactNode; label: string }> = {
  healthy:  { color: '#52c41a', icon: <CheckCircleOutlined />, label: 'Healthy' },
  degraded: { color: '#faad14', icon: <ExclamationCircleOutlined />, label: 'Degraded' },
  unhealthy:{ color: '#ff4d4f', icon: <CloseCircleOutlined />, label: 'Unhealthy' },
  unknown:  { color: '#d9d9d9', icon: <QuestionCircleOutlined />, label: 'Unknown' },
};

interface Props {
  status: HealthStatus;
  showLabel?: boolean;
  size?: 'small' | 'default';
}

export default function HealthStatusIndicator({ status, showLabel = false, size = 'default' }: Props) {
  const cfg = STATUS_CONFIG[status] || STATUS_CONFIG.unknown;
  const dotSize = size === 'small' ? 6 : 8;

  return (
    <Tooltip title={cfg.label}>
      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, cursor: 'default' }}>
        <Badge
          color={cfg.color}
          dot
          style={{ width: dotSize, height: dotSize }}
        />
        {showLabel && (
          <span style={{ fontSize: 12, color: cfg.color, fontWeight: 500 }}>
            {cfg.label}
          </span>
        )}
      </span>
    </Tooltip>
  );
}
