/**
 * Health Check — 一键健康检查页面 (v1.8G enhanced).
 */

import { useState } from 'react';
import { system, systemHealthApi, portCheckApi, healthCheckApi } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, Alert, StatusBadge } from '@/components/shared';

function fmtDisk(bytes: number) {
  if (!bytes) return '—';
  const gb = bytes / (1024 * 1024 * 1024);
  return gb >= 1 ? `${gb.toFixed(1)} GB` : `${(bytes / (1024 * 1024)).toFixed(0)} MB`;
}

export default function HealthCheckPage() {
  const [status, setStatus] = useState<any>(null);
  const [sysHealth, setSysHealth] = useState<any>(null);
  const [portConflicts, setPortConflicts] = useState<any>(null);
  const [healthResults, setHealthResults] = useState<any>(null);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function doCheck() {
    setChecking(true);
    setError(null);
    setStatus(null);
    setSysHealth(null);
    setPortConflicts(null);
    setHealthResults(null);
    try {
      const [sysRes, sysH, ports, hc] = await Promise.all([
        system.status(),
        systemHealthApi.get().catch(() => null),
        portCheckApi.scan().catch(() => null),
        healthCheckApi.getLatest().catch(() => null),
      ]);
      setStatus(sysRes);
      setSysHealth(sysH);
      setPortConflicts(ports);
      setHealthResults(hc);
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

      {!status && !error && (
        <Card>
          <div className="p-[18px] text-xs text-a-muted">
            点击「开始检查」查看系统各组件的健康状态——包括端点健康、SQLite 完整性、磁盘/内存、端口冲突。
          </div>
        </Card>
      )}

      {status && (
        <div className="space-y-4">
          {/* Endpoint health counts */}
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

          {/* System health metrics */}
          {sysHealth && (
            <Card title="系统资源">
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3 p-3">
                <div className="text-xs">
                  <div className="text-a-muted mb-0.5">SQLite</div>
                  <div className="flex items-center gap-1.5">
                    <span className={`w-1.5 h-1.5 rounded-full ${sysHealth.sqlite_ok ? 'bg-[#4cd964]' : 'bg-[#ff5c72]'}`} />
                    <span className="font-medium">{sysHealth.sqlite_ok ? '正常' : '异常'}</span>
                  </div>
                  <div className="text-[10px] text-a-muted mt-0.5">{fmtDisk(sysHealth.sqlite_size_bytes)}</div>
                </div>
                <div className="text-xs">
                  <div className="text-a-muted mb-0.5">磁盘可用</div>
                  <div className="font-medium">{fmtDisk(sysHealth.disk_free_bytes)}</div>
                  <div className="text-[10px] text-a-muted mt-0.5">/ {fmtDisk(sysHealth.disk_total_bytes)}</div>
                </div>
                <div className="text-xs">
                  <div className="text-a-muted mb-0.5">内存</div>
                  <div className="font-medium">{sysHealth.memory_used_mb} MB</div>
                  <div className="text-[10px] text-a-muted mt-0.5">/ {sysHealth.memory_total_mb} MB</div>
                </div>
                <div className="text-xs">
                  <div className="text-a-muted mb-0.5">运行时间</div>
                  <div className="font-medium">
                    {sysHealth.uptime_seconds > 3600
                      ? `${Math.floor(sysHealth.uptime_seconds / 3600)}h ${Math.floor((sysHealth.uptime_seconds % 3600) / 60)}m`
                      : `${Math.floor(sysHealth.uptime_seconds / 60)}m`}
                  </div>
                  <div className="text-[10px] text-a-muted mt-0.5">{sysHealth.go_version} · {sysHealth.goroutines} goroutines</div>
                </div>
              </div>
            </Card>
          )}

          {/* Port conflicts */}
          {portConflicts && (
            <Card title={`端口扫描 · ${portConflicts.total || 0} 未管理端口`}>
              {portConflicts.conflicts?.length > 0 ? (
                <div className="overflow-x-auto">
                  <table className="w-full text-xs">
                    <thead>
                      <tr className="border-b border-a-border text-a-muted text-left">
                        <th className="py-2 px-3 font-medium">端口</th>
                        <th className="py-2 px-3 font-medium">绑定 IP</th>
                        <th className="py-2 px-3 font-medium">状态</th>
                        <th className="py-2 px-3 font-medium">信息</th>
                      </tr>
                    </thead>
                    <tbody>
                      {portConflicts.conflicts.map((c: any, i: number) => (
                        <tr key={i} className="border-b border-a-border/50">
                          <td className="py-2 px-3 font-mono text-a-fg">{c.port}</td>
                          <td className="py-2 px-3 font-mono text-a-muted">{c.bind_ip || '0.0.0.0'}</td>
                          <td className="py-2 px-3"><StatusBadge status={c.status} /></td>
                          <td className="py-2 px-3 text-a-muted text-[11px]">{c.message || '—'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className="text-xs text-a-muted p-3">✓ 未检测到未管理的端口占用</div>
              )}
            </Card>
          )}

          {/* System info */}
          <Card title="系统信息">
            <div className="p-[18px] grid grid-cols-2 gap-3 text-xs">
              <div><span className="text-a-muted">服务器:</span> {status.server_time}</div>
              <div><span className="text-a-muted">提供商:</span> {status.proxy?.provider || '—'}</div>
              <div><span className="text-a-muted">配置:</span> {status.proxy?.config_path || '—'}</div>
              <div><span className="text-a-muted">模式版本:</span> {status.store?.schema_version || '—'}</div>
            </div>
          </Card>
        </div>
      )}
    </div>
  );
}
