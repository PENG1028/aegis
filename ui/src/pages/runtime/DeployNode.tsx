// ─── DeployNode ───
// SSH deployment form for installing Aegis on a remote machine.
//
// @design: This file serves as a reference for building deployment UIs
// in future distributed systems. Patterns to note:
//   1. Three auth modes: SSH Key / SSH Password / Join Token
//   2. Real-time log output from the deploy API
//   3. Form validation mirrors backend validation
//   4. Error states are inline, not alerts
//
// Backend API: POST /api/admin/v1/nodes/deploy
// See internal/httpapi/handlers/deploy_node.go for the matching handler.

import { useState, useRef, useEffect } from 'react';
import { Card, PageHeader, Btn, useToast, LoadingState } from '@/components/shared';
import Input from '@/components/ui/Input';
import { adminApi } from '@/lib/api-bridge';

type AuthMode = 'key' | 'password' | 'token';

interface DeployForm {
  targetIp: string;
  sshUser: string;
  sshPort: string;
  authMode: AuthMode;
  sshKey: string;
  sshPassword: string;
  joinToken: string;
}

export default function DeployNode() {
  const toast = useToast();
  const logEndRef = useRef<HTMLDivElement>(null);

  // ── Form state ──
  const [form, setForm] = useState<DeployForm>({
    targetIp: '',
    sshUser: 'ubuntu',
    sshPort: '22',
    authMode: 'key',
    sshKey: '',
    sshPassword: '',
    joinToken: '',
  });

  const [deploying, setDeploying] = useState(false);
  const [logs, setLogs] = useState<string[]>([]);
  const [result, setResult] = useState<{ success: boolean; message: string; manualCommand?: string } | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Auto-scroll logs
  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [logs]);

  // ── Validation ──
  const validationError = (): string | null => {
    if (!form.targetIp.trim()) return '请输入目标 IP 地址';
    if (form.authMode === 'key' && !form.sshKey.trim()) return '请粘贴 SSH 私钥或选择文件';
    if (form.authMode === 'password' && !form.sshPassword.trim()) return '请输入 SSH 密码';
    if (form.authMode === 'token' && !form.joinToken.trim()) return '请输入 Join Token';
    return null;
  };

  // ── Deploy ──
  const handleDeploy = async () => {
    const ve = validationError();
    if (ve) { toast(ve, 'error'); return; }

    setDeploying(true);
    setLogs(['开始部署...']);
    setResult(null);
    setError(null);

    try {
      const res = await adminApi.deployNode({
        target_ip: form.targetIp,
        ssh_user: form.sshUser,
        ssh_port: parseInt(form.sshPort) || 22,
        auth_method: form.authMode,
        ssh_key: form.authMode === 'key' ? form.sshKey : undefined,
        ssh_password: form.authMode === 'password' ? form.sshPassword : undefined,
        join_token: form.joinToken || undefined,
      });

      if (res.log_output) {
        setLogs(res.log_output.split('\n').filter(Boolean));
      }

      if (res.success) {
        setResult({ success: true, message: res.message });
        toast('部署成功');
      } else if (res.manual_command) {
        setResult({ success: false, message: res.message, manualCommand: res.manual_command });
      } else {
        setResult({ success: false, message: res.message || '部署失败' });
      }
    } catch (err: any) {
      setError(err.message || '部署请求失败');
    } finally {
      setDeploying(false);
    }
  };

  // ── Render ──
  return (
    <div className="p-6 space-y-6">
      <PageHeader title="部署节点" subtitle="SSH 远程安装 Aegis 到新机器" />

      {/* ── Deployment form ──
           @design: Form fields map 1:1 to the backend DeployNodeRequest.
           Auth mode toggles which fields are visible. */}
      <Card title="SSH 部署">
        <div className="space-y-3 max-w-lg">

          {/* Target address */}
          <div className="grid grid-cols-3 gap-2">
            <div className="col-span-2">
              <label className="text-xs text-a-muted block mb-1">SSH 地址</label>
              <Input value={form.targetIp} onChange={e => setForm({...form, targetIp: e.target.value})}
                placeholder="192.168.1.100" />
            </div>
            <div>
              <label className="text-xs text-a-muted block mb-1">端口</label>
              <Input value={form.sshPort} onChange={e => setForm({...form, sshPort: e.target.value})}
                placeholder="22" />
            </div>
          </div>
          <div>
            <label className="text-xs text-a-muted block mb-1">SSH 用户</label>
            <Input value={form.sshUser} onChange={e => setForm({...form, sshUser: e.target.value})}
              placeholder="ubuntu" />
          </div>

          {/* Auth mode selector */}
          <div>
            <label className="text-xs text-a-muted block mb-2">认证方式</label>
            <div className="flex gap-4">
              {([
                { value: 'key' as AuthMode, label: 'SSH Key' },
                { value: 'password' as AuthMode, label: 'SSH 密码' },
                { value: 'token' as AuthMode, label: 'Join Token' },
              ]).map(m => (
                <label key={m.value} className="flex items-center gap-1.5 text-xs cursor-pointer">
                  <input type="radio" name="auth" value={m.value}
                    checked={form.authMode === m.value}
                    onChange={() => setForm({...form, authMode: m.value})}
                    className="accent-a-accent" />
                  {m.label}
                </label>
              ))}
            </div>
          </div>

          {/* SSH Key input */}
          {form.authMode === 'key' && (
            <div>
              <label className="text-xs text-a-muted block mb-1">SSH 私钥</label>
              <textarea value={form.sshKey} onChange={e => setForm({...form, sshKey: e.target.value})}
                placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                className="w-full h-24 px-3 py-2 rounded-a-sm bg-a-bg border border-a-border/50 text-xs font-mono text-a-fg placeholder:text-a-muted/30 focus:outline-none focus:border-a-accent/50" />
              <div className="text-[10px] text-a-muted mt-1">
                粘贴私钥内容，或 <span className="text-a-accent cursor-pointer hover:underline"
                  onClick={() => {
                    const el = document.createElement('input');
                    el.type = 'file';
                    el.accept = '.pem,.key,.txt';
                    el.onchange = (e: any) => {
                      const file = e.target.files?.[0];
                      if (!file) return;
                      const reader = new FileReader();
                      reader.onload = () => setForm({...form, sshKey: reader.result as string});
                      reader.readAsText(file);
                    };
                    el.click();
                  }}>选择文件</span>
              </div>
            </div>
          )}

          {/* SSH Password input */}
          {form.authMode === 'password' && (
            <div>
              <label className="text-xs text-a-muted block mb-1">SSH 密码</label>
              <input type="password" value={form.sshPassword}
                onChange={e => setForm({...form, sshPassword: e.target.value})}
                placeholder="••••••••"
                className="w-full px-3 py-2 rounded-a-sm bg-a-bg border border-a-border/50 text-sm text-a-fg placeholder:text-a-muted/30 focus:outline-none focus:border-a-accent/50" />
              <div className="text-[10px] text-a-muted mt-1">需要目标机器安装 sshpass</div>
            </div>
          )}

          {/* Join Token input (always shown, but required only in token mode) */}
          <div>
            <label className="text-xs text-a-muted block mb-1">
              Join Token {form.authMode !== 'token' && <span className="text-a-muted/50">（可选）</span>}
            </label>
            <div className="flex gap-2">
              <Input value={form.joinToken} onChange={e => setForm({...form, joinToken: e.target.value})}
                placeholder={form.authMode === 'token' ? 'jt_abc123...' : '留空则跳过注册'} className="flex-1" />
              {form.authMode === 'token' && (
                <Btn onClick={() => window.open('/access/tokens', '_blank')} className="whitespace-nowrap">
                  新建 Token
                </Btn>
              )}
            </div>
          </div>

          {/* Error message */}
          {error && (
            <div className="text-xs text-[#ff5c72] bg-[#ff5c72]/10 px-3 py-2 rounded-a-sm border border-[#ff5c72]/20">{error}</div>
          )}

          {/* Deploy button */}
          <Btn primary onClick={handleDeploy} disabled={deploying}>
            {deploying ? '部署中...' : '开始部署'}
          </Btn>
        </div>
      </Card>

      {/* ── Deploy log ──
           @design: Shows real-time log output from the deploy API.
           Each [N/7] step marker could be animated with checkmarks
           in a future version. */}
      {logs.length > 0 && (
        <Card title="部署日志">
          <div className="bg-a-bg border border-a-border/30 rounded-a-sm p-3 max-h-64 overflow-y-auto font-mono text-[11px] leading-relaxed">
            {logs.map((line, i) => (
              <div key={i} className={cn(
                line.startsWith('  ') ? 'text-a-muted pl-4' :
                line.startsWith('===') ? 'text-a-accent font-semibold' :
                line.startsWith('✗') || line.startsWith('Warning') ? 'text-[#e8b830]' :
                line.startsWith('✓') ? 'text-[#4cd964]' : 'text-a-fg'
              )}>
                {line}
              </div>
            ))}
            <div ref={logEndRef} />
          </div>
        </Card>
      )}

      {/* ── Result ──
           @design: Success shows a green card with the node info.
           Failure shows the error with the raw log output for debugging.
           Manual command renders a copyable code block. */}
      {result && (
        <Card title={result.success ? '部署成功' : '部署失败'}
          className={result.success ? 'border-[#4cd964]/30' : 'border-[#ff5c72]/30'}>
          {result.success ? (
            <div className="text-xs text-a-fg">
              <div className="flex items-center gap-2 mb-2">
                <span className="w-5 h-5 rounded-full bg-[#4cd964]/20 flex items-center justify-center text-[#4cd964] text-xs">✓</span>
                <span>{result.message}</span>
              </div>
              <div className="text-[10px] text-a-muted">节点将在 30 秒内出现在 <a href="/runtime" className="text-a-accent hover:underline">节点列表</a></div>
            </div>
          ) : result.manualCommand ? (
            <div className="space-y-3">
              <p className="text-xs text-a-muted">{result.message}</p>
              <pre className="bg-a-bg border border-a-border/30 rounded-a-sm p-3 text-[11px] font-mono text-a-fg overflow-x-auto whitespace-pre-wrap">{result.manualCommand}</pre>
              <Btn onClick={() => { navigator.clipboard.writeText(result.manualCommand || ''); toast('已复制'); }} className="text-xs">
                复制命令
              </Btn>
            </div>
          ) : (
            <div className="text-xs text-[#ff5c72]">{result.message}</div>
          )}
        </Card>
      )}

    </div>
  );
}

// cn utility (local copy to avoid import issues)
function cn(...classes: (string | boolean | undefined | null)[]): string {
  return classes.filter(Boolean).join(' ');
}
