import { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { Btn, Card, PageHeader, useToast } from '@/components/shared';
import Input from '@/components/ui/Input';
import { cn } from '@/lib/utils';

type AuthMode = 'key' | 'password';
type ControllerMode = 'current' | 'push_only';
type StepStatus = 'pending' | 'skipped' | 'ok' | 'warning' | 'failed';

interface DeployForm {
  targetIp: string;
  sshUser: string;
  sshPort: string;
  authMode: AuthMode;
  sshKey: string;
  sshPassword: string;
  controllerMode: ControllerMode;
  controlNodeId: string;
  controlEdgeAddr: string;
  controlSecret: string;
}

interface BinaryInfo {
  found: boolean;
  path?: string;
  version?: string;
  running: boolean;
  service?: string;
}

interface PreflightReport {
  host?: { os?: string; arch?: string };
  aegis: BinaryInfo;
  providers?: Record<string, BinaryInfo>;
  config?: { found: boolean; path?: string };
  ports?: { port: number; process: string; listen: string }[];
}

interface PreflightResponse {
  success: boolean;
  error?: string;
  report?: PreflightReport;
}

interface StepReport {
  name: string;
  status: StepStatus;
  message?: string;
}

interface Capability {
  name: string;
  available: boolean;
  detail?: string;
}

interface EnsureResponse {
  success: boolean;
  action?: string;
  node_id?: string;
  peer_addr?: string;
  message?: string;
  error?: string;
  next_step?: string;
  steps?: StepReport[];
  capabilities?: Capability[];
  log_output?: string;
  manual_command?: string;
}

const initialForm: DeployForm = {
  targetIp: '',
  sshUser: 'ubuntu',
  sshPort: '22',
  authMode: 'key',
  sshKey: '',
  sshPassword: '',
  controllerMode: 'current',
  controlNodeId: '',
  controlEdgeAddr: '',
  controlSecret: '',
};

const statusLabel: Record<StepStatus, string> = {
  pending: '等待',
  skipped: '跳过',
  ok: '完成',
  warning: '警告',
  failed: '失败',
};

const statusClass: Record<StepStatus, string> = {
  pending: 'border-a-border text-a-muted bg-a-bg',
  skipped: 'border-a-border text-a-muted bg-a-bg',
  ok: 'border-[#4cd964]/30 text-[#4cd964] bg-[#4cd964]/10',
  warning: 'border-[#e8b830]/30 text-[#e8b830] bg-[#e8b830]/10',
  failed: 'border-[#ff5c72]/30 text-[#ff5c72] bg-[#ff5c72]/10',
};

const actionLabel: Record<string, string> = {
  deploy: '全新部署',
  redeploy: '重新部署',
  join_only: '接入已有节点',
  joined: '已接入',
  deploy_first: '需要先部署',
  start_first: '需要先启动',
  join_failed: '接入失败',
  not_configured: '未配置',
  control_plane_invalid: '控制节点无效',
  push_only_joined: '本机推送接入',
  push_only_deployed: '本机推送部署',
};

const SearchIcon = () => (
  <svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="11" cy="11" r="8" />
    <path d="m21 21-4.3-4.3" />
  </svg>
);

const LinkIcon = () => (
  <svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
    <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
  </svg>
);

const UploadIcon = () => (
  <svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 16V4" />
    <path d="m7 9 5-5 5 5" />
    <path d="M20 16.5V20a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-3.5" />
  </svg>
);

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || data.message || `HTTP ${res.status}`);
  }
  return data as T;
}

function platformText(report?: PreflightReport): string {
  const os = report?.host?.os || 'unknown';
  const arch = report?.host?.arch || 'unknown';
  return `${os}/${arch}`;
}

function splitLog(raw?: string): string[] {
  return (raw || '').split('\n').map(line => line.trimEnd()).filter(Boolean);
}

export default function DeployNode() {
  const toast = useToast();
  const logEndRef = useRef<HTMLDivElement>(null);
  const [form, setForm] = useState<DeployForm>(initialForm);
  const [preflight, setPreflight] = useState<PreflightResponse | null>(null);
  const [result, setResult] = useState<EnsureResponse | null>(null);
  const [logs, setLogs] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [checking, setChecking] = useState(false);
  const [joining, setJoining] = useState(false);
  const [deploying, setDeploying] = useState(false);

  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [logs]);

  const report = preflight?.report;
  const aegisRunning = Boolean(report?.aegis?.running);
  const targetIsClean = Boolean(report && !report.aegis?.found);
  const busy = checking || joining || deploying;
  const primaryAction = useMemo(() => {
    if (!report) return '先检测目标';
    if (aegisRunning) return '接入已有节点';
    return '从零部署节点';
  }, [report, aegisRunning]);

  const validationError = (): string | null => {
    if (!form.targetIp.trim()) return '请输入目标 IP 或主机名';
    if (form.authMode === 'key' && !form.sshKey.trim()) return '请粘贴 SSH 私钥';
    if (form.authMode === 'password' && !form.sshPassword.trim()) return '请输入 SSH 密码';
    if (form.controllerMode === 'push_only' && !form.controlNodeId.trim()) return '请输入公网控制节点 ID';
    if (form.controllerMode === 'push_only' && !form.controlEdgeAddr.trim()) return '请输入公网控制节点地址';
    if (form.controllerMode === 'push_only' && !form.controlSecret.trim()) return '请输入公网集群密钥';
    return null;
  };

  const makeBody = () => ({
    target_ip: form.targetIp.trim(),
    ssh_user: form.sshUser.trim() || 'root',
    ssh_port: Number.parseInt(form.sshPort, 10) || 22,
    auth_method: form.authMode,
    ssh_key: form.authMode === 'key' ? form.sshKey : undefined,
    ssh_password: form.authMode === 'password' ? form.sshPassword : undefined,
    controller_mode: form.controllerMode,
    control_node_id: form.controllerMode === 'push_only' ? form.controlNodeId.trim() : undefined,
    control_edge_addr: form.controllerMode === 'push_only' ? form.controlEdgeAddr.trim() : undefined,
    control_secret: form.controllerMode === 'push_only' ? form.controlSecret.trim() : undefined,
  });

  const runPreflight = async () => {
    const ve = validationError();
    if (ve) {
      toast(ve, 'error');
      return;
    }
    setChecking(true);
    setPreflight(null);
    setResult(null);
    setLogs([]);
    setError(null);
    try {
      const data = await postJSON<PreflightResponse>('/api/admin/v1/nodes/preflight', makeBody());
      setPreflight(data);
      if (!data.success) {
        setError(data.error || '目标检测失败');
        toast(data.error || '目标检测失败', 'error');
      }
    } catch (e: any) {
      setError(e.message || '目标检测请求失败');
    } finally {
      setChecking(false);
    }
  };

  const runEnsure = async (mode: 'join' | 'deploy') => {
    const ve = validationError();
    if (ve) {
      toast(ve, 'error');
      return;
    }
    if (mode === 'join') setJoining(true);
    else setDeploying(true);
    setResult(null);
    setLogs([mode === 'join' ? '开始接入已有节点...' : '开始从零部署节点...']);
    setError(null);
    try {
      const data = await postJSON<EnsureResponse>(`/api/admin/v1/nodes/${mode}`, makeBody());
      setResult(data);
      setLogs(splitLog(data.log_output));
      if (data.success) toast(mode === 'join' ? '节点接入成功' : '节点部署成功');
      else toast(data.error || data.message || '操作失败', 'error');
    } catch (e: any) {
      setError(e.message || '操作请求失败');
    } finally {
      setJoining(false);
      setDeploying(false);
    }
  };

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="部署节点" subtitle="检测目标机器，接入已有 Aegis 节点，或通过 SSH 从零部署新节点。" />

      <Card title="连接信息" subtitle="第一次安装需要 SSH；已安装节点会走接入流程。">
        <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_320px]">
          <div className="space-y-4">
            <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_120px]">
              <Field label="SSH 地址">
                <Input value={form.targetIp} onChange={e => setForm({ ...form, targetIp: e.target.value })} placeholder="43.160.211.232" />
              </Field>
              <Field label="端口">
                <Input value={form.sshPort} onChange={e => setForm({ ...form, sshPort: e.target.value })} placeholder="22" inputMode="numeric" />
              </Field>
            </div>

            <Field label="SSH 用户">
              <Input value={form.sshUser} onChange={e => setForm({ ...form, sshUser: e.target.value })} placeholder="ubuntu" />
            </Field>

            <fieldset className="space-y-2">
              <legend className="text-xs font-medium text-a-muted">控制节点</legend>
              <div className="grid gap-2 sm:grid-cols-2">
                {([
                  { value: 'current', label: '当前控制节点', helper: '当前打开的 Aegis 可以被目标从 80/443 访问。' },
                  { value: 'push_only', label: '本机仅推送', helper: '本机没有公网入口，目标加入另一个公网控制节点。' },
                ] as const).map(option => (
                  <label
                    key={option.value}
                    className={cn(
                      'min-h-16 cursor-pointer rounded-a-sm border px-3 py-2 transition-colors',
                      form.controllerMode === option.value ? 'border-a-accent bg-a-accent/10 text-a-fg' : 'border-a-border bg-a-bg text-a-muted hover:text-a-fg',
                    )}
                  >
                    <div className="flex items-center gap-2 text-xs font-medium">
                      <input
                        type="radio"
                        name="controllerMode"
                        checked={form.controllerMode === option.value}
                        onChange={() => setForm({ ...form, controllerMode: option.value })}
                        className="accent-a-accent"
                      />
                      {option.label}
                    </div>
                    <div className="mt-1 pl-5 text-[11px] leading-relaxed text-a-muted">{option.helper}</div>
                  </label>
                ))}
              </div>
            </fieldset>

            {form.controllerMode === 'push_only' && (
              <div className="grid gap-3 sm:grid-cols-2">
                <Field label="公网控制节点 ID">
                  <Input value={form.controlNodeId} onChange={e => setForm({ ...form, controlNodeId: e.target.value })} placeholder="node_VM-0-11-ubuntu" />
                </Field>
                <Field label="公网控制地址">
                  <Input value={form.controlEdgeAddr} onChange={e => setForm({ ...form, controlEdgeAddr: e.target.value })} placeholder="43.159.34.11:80" />
                </Field>
                <div className="sm:col-span-2">
                  <Field label="公网集群密钥">
                    <input
                      type="password"
                      value={form.controlSecret}
                      onChange={e => setForm({ ...form, controlSecret: e.target.value })}
                      placeholder="DistNode secret / invite secret"
                      className="w-full min-h-10 px-3 py-2 rounded-a-sm bg-a-bg border border-a-border text-sm text-a-fg placeholder:text-a-muted/40 focus:outline-none focus:border-a-accent"
                    />
                  </Field>
                </div>
              </div>
            )}

            <fieldset className="space-y-2">
              <legend className="text-xs font-medium text-a-muted">认证方式</legend>
              <div className="flex flex-wrap gap-2">
                {(['key', 'password'] as AuthMode[]).map(mode => (
                  <label
                    key={mode}
                    className={cn(
                      'min-h-10 inline-flex items-center gap-2 rounded-a-sm border px-3 text-xs cursor-pointer transition-colors',
                      form.authMode === mode ? 'border-a-accent bg-a-accent/10 text-a-fg' : 'border-a-border bg-a-bg text-a-muted hover:text-a-fg',
                    )}
                  >
                    <input
                      type="radio"
                      name="auth"
                      checked={form.authMode === mode}
                      onChange={() => setForm({ ...form, authMode: mode })}
                      className="accent-a-accent"
                    />
                    {mode === 'key' ? 'SSH 私钥' : 'SSH 密码'}
                  </label>
                ))}
              </div>
            </fieldset>

            {form.authMode === 'key' ? (
              <Field label="SSH 私钥">
                <textarea
                  value={form.sshKey}
                  onChange={e => setForm({ ...form, sshKey: e.target.value })}
                  placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                  className="w-full min-h-32 px-3 py-2 rounded-a-sm bg-a-bg border border-a-border text-xs font-mono text-a-fg placeholder:text-a-muted/40 focus:outline-none focus:border-a-accent"
                />
              </Field>
            ) : (
              <Field label="SSH 密码">
                <input
                  type="password"
                  value={form.sshPassword}
                  onChange={e => setForm({ ...form, sshPassword: e.target.value })}
                  placeholder="输入密码"
                  className="w-full min-h-10 px-3 py-2 rounded-a-sm bg-a-bg border border-a-border text-sm text-a-fg placeholder:text-a-muted/40 focus:outline-none focus:border-a-accent"
                />
              </Field>
            )}

            {error && <div className="rounded-a-sm border border-[#ff5c72]/30 bg-[#ff5c72]/10 px-3 py-2 text-xs text-[#ff8a9b]">{error}</div>}

            <div className="flex flex-wrap gap-2">
              <Btn onClick={runPreflight} disabled={busy}>
                <SearchIcon /> {checking ? '检测中...' : '检测目标'}
              </Btn>
              <Btn onClick={() => runEnsure('join')} disabled={busy || !aegisRunning} primary={aegisRunning}>
                <LinkIcon /> {joining ? '接入中...' : '接入已有节点'}
              </Btn>
              <Btn onClick={() => runEnsure('deploy')} disabled={busy} primary={!aegisRunning}>
                <UploadIcon /> {deploying ? '部署中...' : '从零部署'}
              </Btn>
            </div>
          </div>

          <div className="rounded-a-sm border border-a-border bg-a-bg/60 p-4 space-y-3">
            <div className="text-xs font-medium text-a-muted">当前判断</div>
            <div className="text-lg font-semibold text-a-fg">{primaryAction}</div>
            <div className="text-xs text-a-muted leading-relaxed">
              {report ? `目标平台 ${platformText(report)}。${targetIsClean ? '未检测到 Aegis，可以部署。' : aegisRunning ? 'Aegis 正在运行，优先接入。' : '检测到 Aegis，但服务未运行。'}` : '先检测目标，页面会根据目标状态选择接入或部署。'}
            </div>
          </div>
        </div>
      </Card>

      {preflight && (
        <PreflightPanel preflight={preflight} />
      )}

      {result && (
        <EnsureResultPanel result={result} />
      )}

      {logs.length > 0 && (
        <Card title="执行日志">
          <div className="max-h-72 overflow-y-auto rounded-a-sm border border-a-border bg-a-bg p-3 font-mono text-[11px] leading-relaxed">
            {logs.map((line, i) => (
              <div key={`${i}-${line}`} className={cn(line.startsWith('  ') ? 'pl-4 text-a-muted' : line.startsWith('===') ? 'font-semibold text-a-accent' : 'text-a-fg')}>
                {line}
              </div>
            ))}
            <div ref={logEndRef} />
          </div>
        </Card>
      )}
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block space-y-1.5">
      <span className="text-xs font-medium text-a-muted">{label}</span>
      {children}
    </label>
  );
}

function PreflightPanel({ preflight }: { preflight: PreflightResponse }) {
  if (!preflight.success || !preflight.report) {
    return (
      <Card title="检测结果" className="border-[#ff5c72]/30">
        <div className="text-xs text-[#ff8a9b]">{preflight.error || '检测失败'}</div>
      </Card>
    );
  }
  const report = preflight.report;
  const providers = Object.entries(report.providers || {});
  return (
    <Card title="检测结果" subtitle={`目标平台 ${platformText(report)}`}>
      <div className="grid gap-4 lg:grid-cols-3">
        <StatusTile title="Aegis" status={report.aegis.running ? 'ok' : report.aegis.found ? 'warning' : 'pending'} detail={report.aegis.running ? '正在运行' : report.aegis.found ? '已安装但未运行' : '未安装'} />
        <StatusTile title="配置文件" status={report.config?.found ? 'ok' : 'pending'} detail={report.config?.path || '未发现'} />
        <StatusTile title="端口占用" status={(report.ports?.length || 0) > 0 ? 'warning' : 'ok'} detail={(report.ports?.length || 0) > 0 ? `${report.ports?.length} 个端口已有监听` : '未发现关键冲突'} />
      </div>

      {providers.length > 0 && (
        <div className="mt-4 border-t border-a-border pt-4">
          <div className="mb-2 text-xs font-medium text-a-muted">Provider 状态</div>
          <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {providers.map(([id, info]) => (
              <div key={id} className="rounded-a-sm border border-a-border bg-a-bg px-3 py-2">
                <div className="flex items-center justify-between gap-2">
                  <span className="font-mono text-xs text-a-fg">{id}</span>
                  <StepBadge status={info.found ? 'ok' : 'warning'} />
                </div>
                <div className="mt-1 text-[11px] text-a-muted">{info.version || (info.found ? info.path : '目标缺少该 provider')}</div>
              </div>
            ))}
          </div>
        </div>
      )}

      {(report.ports?.length || 0) > 0 && (
        <div className="mt-4 border-t border-a-border pt-4">
          <div className="mb-2 text-xs font-medium text-a-muted">监听端口</div>
          <div className="space-y-2">
            {report.ports?.map(port => (
              <div key={`${port.port}-${port.listen}`} className="flex flex-wrap items-center gap-2 rounded-a-sm border border-a-border bg-a-bg px-3 py-2 text-xs">
                <span className="font-mono text-a-fg">:{port.port}</span>
                <span className="text-a-muted">{port.process}</span>
                <span className="font-mono text-[11px] text-a-muted">{port.listen}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </Card>
  );
}

function EnsureResultPanel({ result }: { result: EnsureResponse }) {
  const failed = !result.success;
  return (
    <Card
      title={result.success ? '操作完成' : '操作未完成'}
      subtitle={result.action ? actionLabel[result.action] || result.action : undefined}
      className={failed ? 'border-[#ff5c72]/30' : 'border-[#4cd964]/30'}
      actions={result.node_id && <Link to="/runtime" className="text-xs text-a-accent hover:underline">查看节点</Link>}
    >
      <div className="space-y-4">
        <div className={cn('rounded-a-sm border px-3 py-2 text-xs', failed ? 'border-[#ff5c72]/30 bg-[#ff5c72]/10 text-[#ff8a9b]' : 'border-[#4cd964]/30 bg-[#4cd964]/10 text-[#4cd964]')}>
          {result.message || result.error || (result.success ? '节点已准备完成' : '操作失败')}
        </div>

        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {result.node_id && <MetaTile label="node_id" value={result.node_id} />}
          {result.peer_addr && <MetaTile label="peer_addr" value={result.peer_addr} />}
          {result.next_step && <MetaTile label="next_step" value={result.next_step} />}
        </div>

        {(result.steps?.length || 0) > 0 && (
          <div>
            <div className="mb-2 text-xs font-medium text-a-muted">执行步骤</div>
            <div className="space-y-2">
              {result.steps?.map((step, i) => (
                <div key={`${step.name}-${i}`} className="grid gap-2 rounded-a-sm border border-a-border bg-a-bg px-3 py-2 sm:grid-cols-[32px_minmax(0,1fr)_80px] sm:items-center">
                  <div className="font-mono text-[11px] text-a-muted">{String(i + 1).padStart(2, '0')}</div>
                  <div className="min-w-0">
                    <div className="font-mono text-xs text-a-fg">{step.name}</div>
                    {step.message && <div className="mt-0.5 text-[11px] text-a-muted break-words">{step.message}</div>}
                  </div>
                  <StepBadge status={step.status} />
                </div>
              ))}
            </div>
          </div>
        )}

        {(result.capabilities?.length || 0) > 0 && (
          <div>
            <div className="mb-2 text-xs font-medium text-a-muted">能力检查</div>
            <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              {result.capabilities?.map(cap => (
                <div key={cap.name} className="rounded-a-sm border border-a-border bg-a-bg px-3 py-2">
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-mono text-xs text-a-fg">{cap.name}</span>
                    <StepBadge status={cap.available ? 'ok' : 'warning'} />
                  </div>
                  {cap.detail && <div className="mt-1 text-[11px] text-a-muted">{cap.detail}</div>}
                </div>
              ))}
            </div>
          </div>
        )}

        {result.manual_command && (
          <div>
            <div className="mb-2 text-xs font-medium text-a-muted">手动命令</div>
            <pre className="overflow-x-auto whitespace-pre-wrap rounded-a-sm border border-a-border bg-a-bg p-3 font-mono text-[11px] text-a-fg">{result.manual_command}</pre>
          </div>
        )}
      </div>
    </Card>
  );
}

function StatusTile({ title, status, detail }: { title: string; status: StepStatus; detail: string }) {
  return (
    <div className="rounded-a-sm border border-a-border bg-a-bg px-3 py-2">
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs font-medium text-a-fg">{title}</span>
        <StepBadge status={status} />
      </div>
      <div className="mt-1 text-[11px] text-a-muted">{detail}</div>
    </div>
  );
}

function MetaTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-a-sm border border-a-border bg-a-bg px-3 py-2">
      <div className="text-[11px] text-a-muted">{label}</div>
      <div className="mt-1 break-all font-mono text-xs text-a-fg">{value}</div>
    </div>
  );
}

function StepBadge({ status }: { status: StepStatus }) {
  return (
    <span className={cn('inline-flex min-h-6 items-center justify-center rounded-a-sm border px-2 font-mono text-[11px]', statusClass[status])}>
      {statusLabel[status]}
    </span>
  );
}
