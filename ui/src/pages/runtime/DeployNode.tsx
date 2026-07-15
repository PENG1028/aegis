// ─── DeployNode ───
// SSH deployment + preflight detection + node join
//
// Flow: Fill SSH form → [检测目标] → shows status → [连接节点] or [开始部署]

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
    providers: Record<string, { found: boolean; path?: string; version?: string; running: boolean; service?: string }>;
    config: { found: boolean; path?: string };
    ports:  { port: number; process: string; listen: string }[];
  };
}

// SVG icon instead of emoji
const SearchIcon = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="inline-block">
    <circle cx="11" cy="11" r="8" />
    <line x1="21" y1="21" x2="16.65" y2="16.65" />
  </svg>
);

const LinkIcon = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="inline-block">
    <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
    <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
  </svg>
);

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
  const [joining, setJoining] = useState(false);
  const [logs, setLogs] = useState<string[]>([]);
  const [result, setResult] = useState<{ success: boolean; message: string; manualCommand?: string } | null>(null);
  const [joinResult, setJoinResult] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);
  const [autoInstall, setAutoInstall] = useState<string[]>([]);
  const [availableProviders, setAvailableProviders] = useState<{id: string; name: string}[]>([]);

  // Fetch available providers from runtime mode
  useEffect(() => {
    fetch('/api/system/runtime-mode').then(r => r.json()).then(data => {
      const current = data.current;
      if (current?.provider_ids) {
        // Map provider IDs to labels from the provider list
        fetch('/api/admin/v1/providers').then(r => r.json()).then(pd => {
          const provs = (pd?.providers || []).filter(
            (p: any) => current.provider_ids.includes(p.id)
          ).map((p: any) => ({ id: p.id, name: p.name || p.id }));
          setAvailableProviders(provs);
        }).catch(() => {
          // Fallback: just use provider_ids as labels
          setAvailableProviders(current.provider_ids.map((id: string) => ({ id, name: id })));
        });
      }
    }).catch(() => {});
  }, []);

  const toggleProvider = (id: string) => {
    setAutoInstall(prev => prev.includes(id) ? prev.filter(p => p !== id) : [...prev, id]);
  };

  useEffect(() => { logEndRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [logs]);

  const validationError = (): string | null => {
    if (!form.targetIp.trim()) return '请输入目标 IP 地址';
    if (form.authMode === 'key' && !form.sshKey.trim()) return '请粘贴 SSH 私钥或选择文件';
    if (form.authMode === 'password' && !form.sshPassword.trim()) return '请输入 SSH 密码';
    if (form.authMode === 'token' && !form.joinToken.trim()) return '请输入 Join Token';
    return null;
  };

  const makeBody = () => ({
    target_ip: form.targetIp, ssh_user: form.sshUser,
    ssh_port: parseInt(form.sshPort) || 22, auth_method: form.authMode,
    ssh_key: form.authMode === 'key' ? form.sshKey : undefined,
    ssh_password: form.authMode === 'password' ? form.sshPassword : undefined,
  });

  // ── Preflight ──
  const handlePreflight = async () => {
    const ve = validationError();
    if (ve) { toast(ve, 'error'); return; }
    setChecking(true); setPreflight(null); setError(null); setJoinResult(null);
    try {
      const res = await fetch('/api/admin/v1/nodes/preflight', {
        method: 'POST', credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(makeBody()),
      });
      const data = await res.json();
      setPreflight(data);
      // Auto-enable install for providers NOT already found
      if (data.report?.providers) {
        const missing = Object.keys(data.report.providers).filter(
          (id: string) => !data.report.providers[id]?.found
        );
        if (missing.length > 0) setAutoInstall(missing);
      }
      if (!data.success) toast(data.error || '检测失败', 'error');
    } catch (e: any) {
      setError(e.message || '预检请求失败');
    } finally { setChecking(false); }
  };

  // ── Join Node ──
  const handleJoinNode = async () => {
    setJoining(true); setJoinResult(null); setError(null);
    try {
      const res = await fetch('/api/admin/v1/nodes/join', {
        method: 'POST', credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(makeBody()),
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
        body: JSON.stringify({ ...makeBody(), auto_install: autoInstall }),
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

  // ── Derived state from preflight ──
  const r = preflight?.report;
  const aegisRunning = r?.aegis?.running;
  const isClean = r && !r.aegis.found;

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="部署节点" subtitle="SSH 远程安装 · 预检冲突 · 节点加入" />

      {/* ── SSH Form ── */}
      <Card title="SSH 连接">
        <div className="space-y-3 max-w-lg">
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
          <Btn onClick={handlePreflight} disabled={checking} className="w-full">
            <SearchIcon /> {checking ? '检测中...' : '检测目标'}
          </Btn>
        </div>
      </Card>

      {/* ── Preflight Result ── */}
      {preflight && preflight.success && r && (
        <Card title={isClean ? '✅ 目标就绪' : aegisRunning ? '🟢 Aegis 运行中' : '⚠ 目标状态'}
          className={isClean ? 'border-[#4cd964]/30' : aegisRunning ? 'border-[#4cd964]/30' : 'border-[#e8b830]/30'}>
          <div className="space-y-2 text-xs">
            {isClean && <div className="text-[#4cd964]">✅ 目标为空机器，可全新部署</div>}
            {r.aegis.found && (
              <div className={aegisRunning ? 'text-[#4cd964]' : 'text-a-fg'}>
                {aegisRunning ? '🟢' : '🟡'} Aegis {r.aegis.version || '未知版本'} — {aegisRunning ? '运行中' : '已停止'}
              </div>
            )}
            {/* Per-provider detection results (not hardcoded to Caddy) */}
            {r.providers && Object.entries(r.providers).map(([id, info]: [string, any]) => (
              <div key={id} className={info.found ? 'text-[#4cd964]' : 'text-[#e8b830]'}>
                {info.found ? '✓' : '⚠'} {id} {info.version || ''}
                {info.found ? '' : ' — 建议一键安装'}
              </div>
            ))}
            {r.config.found && <div className="text-a-muted">📄 配置: {r.config.path}</div>}
            {r.ports?.filter((p: any) => !availableProviders.some(ap => ap.id === p.process)).map((p, i) => (
              <div key={i} className="text-[#ff5c72] bg-[#ff5c72]/5 px-2 py-1 rounded">
                🔴 端口 :{p.port} 被 {p.process} 占用
              </div>
            ))}
          </div>

          {/* ── Auto-install provider select ──}
          {availableProviders.length > 0 && (
            <div className="mt-3 pt-3 border-t border-a-border/30">
              <label className="text-xs text-a-fg font-medium block mb-2">一键安装中间件（部署后自动安装）</label>
              <div className="flex flex-wrap gap-2">
                {availableProviders.map(ap => (
                  <label key={ap.id} className={cn(
                    "flex items-center gap-1.5 px-2 py-1 rounded text-xs cursor-pointer border transition-colors",
                    autoInstall.includes(ap.id) ? "bg-a-accent/10 border-a-accent/30 text-a-accent" : "bg-a-bg border-a-border text-a-muted hover:text-a-fg"
                  )}>
                    <input type="checkbox" className="sr-only"
                      checked={autoInstall.includes(ap.id)}
                      onChange={() => toggleProvider(ap.id)} />
                    {ap.name}
                  </label>
                ))}
              </div>
              {autoInstall.length > 0 && (
                <div className="text-[10px] text-a-muted mt-1">
                  部署完成后，将在目标节点上安装选中的中间件
                </div>
              )}
            </div>
          )}
          {/* ── Actions — merged here, not in separate card ── */}
          <div className="mt-4 pt-3 border-t border-a-border/30 space-y-2">
            {error && (
              <div className="text-xs text-[#ff5c72] bg-[#ff5c72]/10 px-3 py-2 rounded-a-sm border border-[#ff5c72]/20">{error}</div>
            )}

            {aegisRunning ? (
              <>
                <Btn onClick={handleJoinNode} disabled={joining} primary className="w-full">
                  <LinkIcon /> {joining ? '加入中...' : '连接节点（加入集群）'}
                </Btn>
                <Btn onClick={handleDeploy} disabled={deploying} className="w-full text-a-muted text-xs">
                  {deploying ? '部署中...' : '覆盖部署（重新安装）'}
                </Btn>
              </>
            ) : (
              <Btn onClick={handleDeploy} disabled={deploying} primary className="w-full">
                {deploying ? '部署中...' : '开始部署'}
              </Btn>
            )}

            {joinResult && (
              <div className={joinResult.success ? 'text-[#4cd964] text-xs' : 'text-[#ff5c72] text-xs'}>
                {joinResult.success ? '✅ ' + joinResult.message : '❌ ' + (joinResult.error || '失败')}
                {joinResult.next_step && <div className="text-a-muted mt-1">{joinResult.next_step}</div>}
              </div>
            )}
          </div>
        </Card>
      )}

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
