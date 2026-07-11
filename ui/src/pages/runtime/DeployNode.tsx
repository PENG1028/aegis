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
  report?: {
    aegis:  { found: boolean; path?: string; version?: string; running: boolean; service?: string };
    caddy:  { found: boolean; path?: string; version?: string; running: boolean; service?: string };
    config: { found: boolean; path?: string };
    ports:  { port: number; process: string; listen: string }[];
  };
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

  // ── Join Node ──
  const [joining, setJoining] = useState(false);
  const [joinResult, setJoinResult] = useState<any>(null);
  const handleJoinNode = async () => {
    const ve = validationError();
    if (ve) { toast(ve, 'error'); return; }
    setJoining(true); setJoinResult(null); setError(null);
    try {
      const res = await fetch('/api/admin/v1/nodes/join', {
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
      setJoinResult(data);
      if (data.success) toast('节点加入成功');
      else toast(data.error || '加入失败', 'error');
    } catch (e: any) {
      setError(e.message || '节点加入请求失败');
    } finally { setJoining(false); }
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
      {preflight && preflight.success && preflight.report && (() => {
        const r = preflight.report;
        const warnings = [];
        if (r.aegis.found) warnings.push(`Aegis ${r.aegis.version || ''} ${r.aegis.running ? '运行中' : '已安装'} — 建议节点加入`);
        if (r.caddy.found) warnings.push(`Caddy ${r.caddy.version || ''} ${r.caddy.running ? '运行中' : '已安装'} — 可跳过安装`);
        if (r.config.found) warnings.push('配置文件已存在 — 部署将覆盖');
        const portWarnings = r.ports?.filter(p => !r.caddy?.running || p.process !== 'caddy') || [];
        const hasWarnings = warnings.length > 0 || portWarnings.length > 0;
        const isClean = !r.aegis.found && !r.caddy.found;

        return (
          <Card title={isClean ? '✅ 目标就绪' : hasWarnings ? '⚠ 冲突检测' : '✅ 目标就绪'}
            className={isClean ? 'border-[#4cd964]/30' : 'border-[#e8b830]/30'}>
            <div className="space-y-2 text-xs">
              {isClean && <div className="text-[#4cd964]">✅ 目标为空机器，可全新部署</div>}
              {r.aegis.found && (
                <div className="space-y-2">
                  <div className={r.aegis.running ? 'text-[#e8b830]' : 'text-a-fg'}>
                    {r.aegis.running ? '🟢' : '🟡'} Aegis {r.aegis.version || '未知版本'} — {r.aegis.running ? '运行中' : '已停止'}
                  </div>
                  {r.aegis.running && (
                    <Btn onClick={() => handleJoinNode()} disabled={joining} className="text-xs" primary>
                      {joining ? '加入中...' : '🔗 连接节点（加入集群）'}
                    </Btn>
                  )}
                  {joinResult && <div className="text-xs mt-2">
                    <div className={joinResult.success ? 'text-[#4cd964]' : 'text-[#ff5c72]'}>
                      {joinResult.success ? '✅ ' + joinResult.message : '❌ ' + (joinResult.error || '失败')}
                    </div>
                    {joinResult.next_step && <div className="text-a-muted mt-1">{joinResult.next_step}</div>}
                  </div>}
                </div>
              )}
              {r.caddy.found && (
                <label className="flex items-center gap-2 text-a-muted cursor-pointer">
                  <input type="checkbox" checked={skipCaddy} onChange={e => setSkipCaddy(e.target.checked)} />
                  Caddy {r.caddy.version || ''} {r.caddy.running ? '🟢 运行中' : ''} — 跳过安装
                </label>
              )}
              {r.config.found && <div className="text-[#e8b830]">📄 配置: {r.config.path}</div>}
              {portWarnings.map((p, i) => (
                <div key={i} className="text-[#ff5c72] bg-[#ff5c72]/5 px-2 py-1 rounded">
                  🔴 端口 :{p.port} 被 {p.process} 占用 — 部署前需停止或迁移
                </div>
              ))}
            </div>
          </Card>
        );
      })()}

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
