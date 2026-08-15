import { Card, PageHeader, Btn, useToast } from '@/components/shared';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { adminApi } from '@/lib/api-bridge';

export default function Rollback() {
  const toast = useToast();
  const [confirmText, setConfirmText] = useState('');
  const [loading, setLoading] = useState(false);

  // Real apply history — the page previously showed hardcoded "v43 → v42".
  const { data } = useQuery({
    queryKey: ['apply-history-rollback'],
    queryFn: () => adminApi.applyHistory().catch(() => [] as any[]),
  });
  const history: any[] = Array.isArray(data) ? data : (data as any)?.history || [];
  const lastSuccess = history.find((h: any) => h.status === 'success') || null;

  const handleRollback = async () => {
    if (confirmText !== 'ROLLBACK') return;
    setLoading(true);
    try {
      await adminApi.rollback();
      toast('回滚成功');
    } catch (e: any) {
      toast(e.message || '回滚失败', 'error');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="回滚配置" subtitle="恢复到上一个成功版本 · 高风险操作" />
      <Card title="回滚确认">
        <div className="space-y-4 max-w-md">
          <div className="p-3 rounded-a-sm bg-[#ff5c72]/5 border border-[#ff5c72]/20">
            <p className="text-xs text-[#ff5c72] font-medium mb-2">⚠️ 此操作将回滚到上一个成功发布的配置</p>
            {lastSuccess ? (
              <p className="text-xs text-a-muted">
                将恢复到: <span className="font-mono text-a-fg">{lastSuccess.version || '未知版本'}</span>
                <span className="ml-2 text-a-border/60">({new Date(lastSuccess.created_at || '').toLocaleString('zh-CN')})</span>
              </p>
            ) : (
              <p className="text-xs text-a-muted">暂无历史成功记录（执行过一次 Apply 后这里会显示真实版本）</p>
            )}
          </div>
          <div>
            <label className="text-xs text-a-muted block mb-1">输入 ROLLBACK 以确认</label>
            <input className="w-full px-3 py-1.5 text-xs rounded-a-md bg-a-bg border border-a-border text-a-fg" value={confirmText} onChange={e => setConfirmText(e.target.value)} />
          </div>
          <Btn danger disabled={confirmText !== 'ROLLBACK' || loading || !lastSuccess} onClick={handleRollback}>
            {loading ? '回滚中...' : '确认回滚'}
          </Btn>
        </div>
      </Card>
    </div>
  );
}
