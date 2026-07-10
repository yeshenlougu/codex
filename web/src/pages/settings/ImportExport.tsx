import { useState, useRef } from 'react';
import { Card, Button, Upload, Space, message } from 'antd';
import { UploadOutlined, DownloadOutlined } from '@ant-design/icons';
import { importBackendsFile, getBackendsExportUrl } from '../../lib/api';
import type { ImportResult } from '../../lib/types';

export default function ImportExport() {
  const [result, setResult] = useState<ImportResult | null>(null);

  const doImport = async (file: File) => {
    message.loading({ content: 'Importing...', key: 'import' });
    try {
      const res = await importBackendsFile(file);
      setResult(res);
      message.success({ content: `Imported ${res.count} backends`, key: 'import' });
    } catch (err: any) {
      message.error({ content: err.message, key: 'import' });
      setResult(null);
    }
    return false; // prevent auto-upload
  };

  return (
    <div style={{ maxWidth: 680, display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Card title="Import cc-switch Config" size="small">
        <p style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 12 }}>
          Upload .yaml, .json, or .sql (SQLite dump). Backends will merge into current config.
        </p>
        <Upload accept=".yaml,.yml,.json,.sql,.db" maxCount={1}
          beforeUpload={doImport} showUploadList={false}>
          <Button icon={<UploadOutlined />}>Choose File</Button>
        </Upload>
      </Card>

      {result && (
        <Card title={`✅ ${result.count} backends imported`} size="small" style={{ borderColor: 'var(--green)' }}>
          <p style={{ fontSize: 12, color: 'var(--text-muted)' }}>Strategy: {result.strategy}</p>
          {result.backends.slice(0, 10).map(b => (
            <div key={b.label} style={{ fontSize: 11, color: 'var(--accent)', fontFamily: 'monospace' }}>
              {b.label} → {b.base_url}
            </div>
          ))}
        </Card>
      )}

      <Card title="Export Config" size="small">
        <p style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 12 }}>
          Download current backends in cc-switch YAML format.
        </p>
        <a href={getBackendsExportUrl()} download="cc-switch-export.yaml">
          <Button icon={<DownloadOutlined />}>Download YAML</Button>
        </a>
      </Card>
    </div>
  );
}
