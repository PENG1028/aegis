import { useState } from 'react';
import { auth } from '@/lib/api-bridge';
import { Card, Btn } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';

export default function ChangePasswordPanel() {
  const toast = useToast();
  const [currentPw, setCurrentPw] = useState('');
  const [newPw, setNewPw] = useState('');
  const [confirmPw, setConfirmPw] = useState('');
  const [saving, setSaving] = useState(false);
  const [result, setResult] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function doChange() {
    setError(null);
    setResult(null);

    if (!currentPw || !newPw || !confirmPw) {
      setError('所有字段不能为空');
      return;
    }
    if (newPw !== confirmPw) {
      setError('两次输入的新密码不一致');
      return;
    }
    if (newPw.length < 8) {
      setError('新密码至少 8 个字符');
      return;
    }

    setSaving(true);
    try {
      const res = await auth.changePassword(currentPw, newPw);
      setResult(res.message || '密码已修改');
      setCurrentPw('');
      setNewPw('');
      setConfirmPw('');
      toast('密码修改成功');
    } catch (e: any) {
      setError(e.message || '修改失败');
    }
    setSaving(false);
  }

  return (
    <Card title="修改密码">
      <div className="p-[18px] space-y-3">
        <div>
          <label className="block text-xs font-medium text-a-muted mb-1">当前密码</label>
          <input
            type="password"
            className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
            value={currentPw}
            onChange={(e) => setCurrentPw(e.target.value)}
            placeholder="输入当前密码"
          />
        </div>
        <div className="flex gap-3">
          <div className="flex-1">
            <label className="block text-xs font-medium text-a-muted mb-1">新密码</label>
            <input
              type="password"
              className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
              value={newPw}
              onChange={(e) => setNewPw(e.target.value)}
              placeholder="至少 8 个字符"
            />
          </div>
          <div className="flex-1">
            <label className="block text-xs font-medium text-a-muted mb-1">确认新密码</label>
            <input
              type="password"
              className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
              value={confirmPw}
              onChange={(e) => setConfirmPw(e.target.value)}
              placeholder="再次输入新密码"
            />
          </div>
        </div>

        {error && <div className="text-xs text-[#ff5c72]">{error}</div>}
        {result && <div className="text-xs text-[#4cd964]">{result}</div>}

        <Btn primary onClick={doChange} disabled={saving}>
          {saving ? '修改中…' : '修改密码'}
        </Btn>
      </div>
    </Card>
  );
}