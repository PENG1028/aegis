// ─── Binaries ───
import { PageHeader, Card, Btn, StatusBadge } from '@/components/shared';

export default function Binaries() {
  return (
    <div className="p-6 space-y-6">
      <PageHeader title="二进制管理" subtitle="上传和管理 Aegis 二进制文件" />
      <Card title="当前版本" subtitle="v1.8L · 2026-07-02 · linux/amd64">
        <div className="flex items-center gap-3 text-xs mb-3">
          <StatusBadge status="active" />
          <span className="text-a-muted">所有节点运行此版本</span>
        </div>
        <Btn primary>上传新版本</Btn>
      </Card>
    </div>
  );
}
