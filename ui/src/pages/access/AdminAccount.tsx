import { Card, PageHeader, Btn } from '@/components/shared';
import { useAuth } from '@/lib/auth-context';
import ChangePasswordPanel from '@/components/settings/ChangePasswordPanel';

export default function AdminAccount() {
  const { user } = useAuth();

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="管理员账号" subtitle={`当前用户: ${user?.username || 'admin'}`} />
      <Card title="账号信息">
        <div className="text-xs space-y-1 mb-4">
          <p><span className="text-a-muted">用户名: </span><span className="text-a-fg">{user?.username || 'admin'}</span></p>
          <p><span className="text-a-muted">ID: </span><span className="font-mono text-a-fg">{user?.id || '1'}</span></p>
        </div>
      </Card>
      <Card title="修改密码">
        <ChangePasswordPanel />
      </Card>
    </div>
  );
}
