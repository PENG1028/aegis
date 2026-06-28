/**
 * Health Check — 一键健康检查页面。
 */

import { useState } from 'react';
import { system } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, Alert, StatusBadge } from '@/components/shared';

export default function HealthCheckPage() {
  const [status, setStatus] = useState<any>(null);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function doCheck() {
    setChecking(true);
    setError(null);
    setStatus(null);
    try {
      const res = await system.status();
      setStatus(res);
    } catch (e: any) {
      setError(e.message);
    }
    setChecking(false);
  }

  return (
    <div>
      <PageHeader title="健康检查" helpKey="health"
        sub="检查系统各组件的健康状态" actions={
          <Btn primary onClick={doCheck} disabled={checking}>
            {checking ? '检查中…' : '开始检查'}
          </Btn>
        } />

      {error && <Alert type="err">{error}</Alert>}

      {status && (
        <div className="space-y-4">
          <div className="grid grid-cols-3 gap-3">
            <Card title="健康端点">
              <div className="p-[18px] text-lg font-bold text-a-success">{status.health?.healthy_endpoints ?? '—'}</div>
            </Card>
            <Card title="异常端点">
              <div className="p-[18px] text-lg font-bold text-a-danger">{status.health?.unhealthy_endpoints ?? '—'}</div>
            </Card>
            <Card title="未知端点">
              <div className="p-[18px] text-lg font-bold text-a-muted">{status.health?.unknown_endpoints ?? '—'}</div>
            </Card>
          </div>
          <Card title="系统状态">
            <div className="p-[18px] grid grid-cols-2 gap-3 text-xs">
              <div><span className="text-a-muted">Server:</span> {status.server_time}</div>
              <div><span className="text-a-muted">Provider:</span> {status.proxy?.provider || '—'}</div>
              <div><span className="text-a-muted">Config:</span> {status.proxy?.config_path || '—'}</div>
              <div><span className="text-a-muted">Schema:</span> {status.store?.schema_version || '—'}</div>
            </div>
          </Card>
        </div>
      )}

      {!status && !error && (
        <Card>
          <div className="p-[18px] text-xs text-a-muted">
            点击「开始检查」查看系统各组件的健康状态。
          </div>
        </Card>
      )}
    </div>
  );
}
