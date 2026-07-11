// ─── DeployNode ───
// SSH deployment + preflight detection + node join
//
// Flow: Fill SSH form → [检测目标] → shows conflicts → [开始部署]

import { useState, useRef, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Card, PageHeader, Btn, useToast } from '@/components/shared';
import Input from '@/components/ui/Input';

type AuthMode = 'key' | 'password' | 'token';

interface DeployForm {
  targetIp: string; sshUser: string; sshPort: string; authMode: AuthMode;
  sshKey: string; sshPassword: string; joinToken: string;
}

interface Preflight {
  success: boolean; error?: string;
  aegis_found?: boolean; aegis_version?: string;
  caddy_found?: boolean; config_found?: boolean;
  has_warnings?: boolean; warnings?: string[];
}

export default function DeployNode() {
  const toast = useToast();
  const logEndRef = useRef<HTMLDivElement>(null);

  const [form, setForm] = useState<DeployForm>({
    targetIp: '', sshUser: 'ubuntu', sshPort: '22', authMode: 'key',
    sshKey: '', sshPassword: '', joinToken: '',
  });

  const [preflight, setPreflight] = useState<Preflight | null>(null);
  const [checking, setChecking] = useState(false);
  const [deploying, setDeploying] = useState(false);
  const [logs, setLogs] = useState<string[]>([]);
  const [result, setResult] = useState<{ success: boolean; message: string; manualCommand?: string } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [skipCaddy, setSkipCaddy] = useState(false);

  useEffect(() => { logEndRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [logs]);

  const validationError = (): string | null => {
    if (!form.targetIp.trim()) return '请输入目标 IP 地址';
    if (form.authMode === 'key' && !form.sshKey.trim()) return '请粘贴 SSH 私钥或选择文件';
    if (form.authMode === 'password' && !form.sshPassword.trim()) return '请输入 SSH 密码';
    if (form.authMode === 'token' && !form.joinToken.trim()) return '请输入 Join Token';
    return null;
  };

  // ── Preflight ──
  const handlePreflight = async () => {
    const ve = validationError();
    if (ve) { toast(ve, 'error'); return; }
    setChecking(true); setPreflight(null); setError(null);
    try {
      const res = await fetch('/api/admin/v1/nodes/preflight', {
        method: 'POST', credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          target_ip: form.targetIp, ssh_user: form.sshUser,
          ssh_port: parseInt(form.sshPort) || 22, auth_method: form.authMode,
          ssh_key: form.authMode === 'key' ? form.sshKey : undefined,
          ssh_password: form.authMode === 'password' ? form.sshPassword : undefined,
        }),
      });
      const data = await res.json();
      setPreflight(data);
      if (data.caddy_found) setSkipCaddy(true);
      if (!data.success) toast(data.error || '检测失败', 'error');
    } catch (e: any) {
      setError(e.message || '预检请求失败');
    } finally { setChecking(false); }
  };

  // ── Deploy ──
  const handleDeploy = async () => {
    const ve = validationError();
    if (ve) { toast(ve, 'error'); return; }
    setDeploying(true); setLogs(['开始部署...']); setResult(null); setError(null);
    try {
      const res = await fetch('/api/admin/v1/nodes/deploy', {
        method: 'POST', credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          target_ip: form.targetIp, ssh_user: form.sshUser,
          ssh_port: parseInt(form.sshPort) || 22, auth_method: form.authMode,
          ssh_key: form.authMode === 'key' ? form.sshKey : undefined,
          ssh_password: form.authMode === 'password' ? form.sshPassword : undefined,
          join_token: form.joinToken || undefined,
          skip_caddy: skipCaddy,
        }),
      });
      const data = await res.json();
      if (data.log_output) setLogs(data.log_output.split('\n').filter(Boolean));
      if (data.success) {
        setResult({ success: true, message: data.message }); toast('部署成功');
      } else if (data.manual_command) {
        setResult({ success: false, message: data.message, manualCommand: data.manual_command });
      } else {
        setResult({ success: false, message: data.message || '部署失败' });
      }
    } catch (e: any) {
      setError(e.message || '部署请求失败');
    } finally { setDeploying(false); }
  };

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="部署节点" subtitle="SSH 远程安装 · 预检冲突 · 节点加入" />

      <Card title="SSH 连接">
        <div className="space-y-3 max-w-lg">
          {/* Target */}
          <div className="grid grid-cols-3 gap-2">
            <div className="col-span-2">
              <label className="text-xs text-a-muted block mb-1">SSH 地址</label>
              <Input value={form.targetIp} onChange={e => setForm({...form, targetIp: e.target.value})}
                placeholder="192.168.1.100" />
            </div>
            <div>
              <label className="text-xs text-a-muted block mb-1">端口</label>
              <Input value={form.sshPort} onChange={e => setForm({...form, sshPort: e.target.value})} placeholder="22" />
            </div>
          </div>
          <div>
            <label className="text-xs text-a-muted block mb-1">SSH 用户</label>
            <Input value={form.sshUser} onChange={e => setForm({...form, sshUser: e.target.value})} placeholder="ubuntu" />
          </div>

          {/* Auth */}
          <div>
            <label className="text-xs text-a-muted block mb-2">认证方式</label>
            <div className="flex gap-4">
              {(['key','password','token'] as AuthMode[]).map(m => (
                <label key={m} className="flex items-center gap-1.5 text-xs cursor-pointer">
                  <input type="radio" name="auth" checked={form.authMode === m}
                    onChange={() => setForm({...form, authMode: m})} className="accent-a-accent" />
                  {m === 'key' ? 'SSH Key' : m === 'password' ? 'SSH 密码' : 'Join Token'}
                </label>
              ))}
            </div>
          </div>

          {form.authMode === 'key' && (
            <div>
              <label className="text-xs text-a-muted block mb-1">SSH 私钥</label>
              <textarea value={form.sshKey} onChange={e => setForm({...form, sshKey: e.target.value})}
                placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                className="w-full h-24 px-3 py-2 rounded-a-sm bg-a-bg border border-a-border/50 text-xs font-mono text-a-fg placeholder:text-a-muted/30 focus:outline-none focus:border-a-accent/50" />
              <div className="text-[10px] text-a-muted mt-1">
                粘贴私钥内容，或 <span className="text-a-accent cursor-pointer hover:underline" onClick={() => {
                  const el = document.createElement('input'); el.type = 'file'; el.accept = '.pem,.key,.txt';
                  el.onchange = (e: any) => { const f = e.target.files?.[0]; if (!f) return; const r = new FileReader(); r.onload = () => setForm({...form, sshKey: r.result as string}); r.readAsText(f); };
                  el.click();
                }}>选择文件</span>
              </div>
            </div>
          )}
          {form.authMode === 'password' && (
            <div>
              <label className="text-xs text-a-muted block mb-1">SSH 密码</label>
              <input type="password" value={form.sshPassword} onChange={e => setForm({...form, sshPassword: e.target.value})}
                placeholder="••••••••" className="w-full px-3 py-2 rounded-a-sm bg-a-bg border border-a-border/50 text-sm text-a-fg placeholder:text-a-muted/30 focus:outline-none focus:border-a-accent/50" />
            </div>
          )}

          <div>
            <label className="text-xs text-a-muted block mb-1">Join Token（可选）</label>
            <Input value={form.joinToken} onChange={e => setForm({...form, joinToken: e.target.value})}
              placeholder="留空则跳过注册" />
          </div>

          {/* Preflight button */}
          <Btn onClick={handlePreflight} disabled={checking} className="w-full">
            {checking ? '检测中...' : '🔍 检测目标'}
          </Btn>
        </div>
      </Card>

      {/* ── Preflight Results ── */}
      {preflight && preflight.success && (
        <Card title={preflight.has_warnings ? '⚠ 冲突检测' : '✅ 目标就绪'} className={preflight.has_warnings ? 'border-[#e8b830]/30' : 'border-[#4cd964]/30'}>
          <div className="space-y-2 text-xs">
            {!preflight.aegis_found && !preflight.caddy_found && (
              <div className="text-[#4cd964]">✅ 目标为空机器，可全新部署</div>
            )}
            {preflight.warnings?.map((w, i) => (
              <div key={i} className="text-[#e8b830]">{w}</div>
            ))}
            {preflight.caddy_found && (
              <label className="flex items-center gap-2 text-a-muted cursor-pointer">
                <input type="checkbox" checked={skipCaddy} onChange={e => setSkipCaddy(e.target.checked)} />
                跳过 Caddy 安装（已存在）
              </label>
            )}
            {preflight.aegis_found && (
              <div className="text-a-fg bg-[#e8b830]/5 border border-[#e8b830]/20 rounded-a-sm p-2">
                目标已有 Aegis {preflight.aegis_version}。建议使用节点加入模式（跳过二进制覆盖），或确认覆盖后继续部署。
              </div>
            )}
          </div>
        </Card>
      )}

      {/* ── Actions ── */}
      <Card title="执行部署">
        <div className="space-y-3 max-w-lg">
          {error && (
            <div className="text-xs text-[#ff5c72] bg-[#ff5c72]/10 px-3 py-2 rounded-a-sm border border-[#ff5c72]/20">{error}</div>
          )}
          <Btn primary onClick={handleDeploy} disabled={deploying}>
            {deploying ? '部署中...' : '开始部署'}
          </Btn>
        </div>
      </Card>

      {/* ── Logs ── */}
      {logs.length > 0 && (
        <Card title="部署日志">
          <div className="bg-a-bg border border-a-border/30 rounded-a-sm p-3 max-h-64 overflow-y-auto font-mono text-[11px] leading-relaxed">
            {logs.map((line, i) => (
              <div key={i} className={cn(
                line.startsWith('  ') ? 'text-a-muted pl-4' :
                line.startsWith('===') ? 'text-a-accent font-semibold' :
                line.startsWith('✗') || line.startsWith('Warning') ? 'text-[#e8b830]' :
                line.startsWith('✓') ? 'text-[#4cd964]' : 'text-a-fg'
              )}>{line}</div>
            ))}
            <div ref={logEndRef} />
          </div>
        </Card>
      )}

      {/* ── Result ── */}
      {result && (
        <Card title={result.success ? '部署成功' : '部署失败'} className={result.success ? 'border-[#4cd964]/30' : 'border-[#ff5c72]/30'}>
          {result.success ? (
            <div className="text-xs text-a-fg">
              <div className="flex items-center gap-2 mb-2">
                <span className="w-5 h-5 rounded-full bg-[#4cd964]/20 flex items-center justify-center text-[#4cd964] text-xs">✓</span>
                <span>{result.message}</span>
              </div>
              <div className="text-[10px] text-a-muted">节点将在 30 秒内出现在 <Link to="/runtime" className="text-a-accent hover:underline">节点列表</Link></div>
            </div>
          ) : result.manualCommand ? (
            <div className="space-y-3">
              <p className="text-xs text-a-muted">{result.message}</p>
              <pre className="bg-a-bg border border-a-border/30 rounded-a-sm p-3 text-[11px] font-mono text-a-fg overflow-x-auto whitespace-pre-wrap">{result.manualCommand}</pre>
              <Btn onClick={() => { navigator.clipboard.writeText(result.manualCommand || ''); toast('已复制'); }} className="text-xs">复制命令</Btn>
            </div>
          ) : (
            <div className="text-xs text-[#ff5c72]">{result.message}</div>
          )}
        </Card>
      )}
    </div>
  );
}

function cn(...classes: (string | boolean | undefined | null)[]): string {
  return classes.filter(Boolean).join(' ');
}
