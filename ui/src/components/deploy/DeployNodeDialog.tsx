import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Modal, Btn } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';
import { API_CONFIG, apiUrl } from '@/lib/api-config';

interface DeployNodeDialogProps {
  open: boolean;
  onClose: () => void;
}

export default function DeployNodeDialog({ open, onClose }: DeployNodeDialogProps) {
  const toast = useToast();
  const qc = useQueryClient();

  const [targetIp, setTargetIp] = useState('');
  const [sshUser, setSshUser] = useState('root');
  const [sshPassword, setSshPassword] = useState('');
  const [joinToken, setJoinToken] = useState('');
  const [cpUrl, setCpUrl] = useState('');
  const [deploying, setDeploying] = useState(false);
  const [logOutput, setLogOutput] = useState('');
  const [manualCmd, setManualCmd] = useState('');

  const handleDeploy = async () => {
    if (!targetIp) { toast('请输入目标 IP', 'error'); return; }
    if (!joinToken) { toast('请输入或生成 Join Token', 'error'); return; }

    setDeploying(true);
    setLogOutput('正在部署...');
    setManualCmd('');

    try {
      const res = await fetch(apiUrl('/api/admin/v1/nodes/deploy'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: API_CONFIG.credentials,
        body: JSON.stringify({
          target_ip: targetIp,
          ssh_user: sshUser || 'root',
          ssh_password: sshPassword,
          join_token: joinToken,
          control_plane_url: cpUrl || window.location.origin,
        }),
      });

      const data = await res.json();

      if (data.success) {
        setLogOutput(data.log_output || data.message);
        toast('节点部署成功！30 秒内将出现在列表中', 'ok');
        qc.invalidateQueries({ queryKey: ['nodes'] });
      } else if (data.manual_command) {
        setManualCmd(data.manual_command);
        setLogOutput('');
        toast('SSH 不可用，请手动运行下方命令', 'error');
      } else {
        setLogOutput(data.log_output || data.message);
        toast(data.message || '部署失败', 'error');
      }
    } catch (err: any) {
      setLogOutput(err.message || '网络错误');
      toast('部署请求失败: ' + (err.message || '网络错误'), 'error');
    } finally {
      setDeploying(false);
    }
  };

  const handleGenerateToken = async () => {
    try {
      const res = await fetch(apiUrl('/api/admin/v1/node-join-tokens'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: API_CONFIG.credentials,
        body: JSON.stringify({ name: `deploy-${targetIp || 'node'}-${Date.now()}` }),
      });
      const data = await res.json();
      if (data.raw_join_token) {
        setJoinToken(data.raw_join_token);
        toast('Join Token 已生成，请妥善保管', 'ok');
      } else if (data.id) {
        toast('Token 已创建但无法显示原始值（仅首次创建时可见）', 'error');
      }
    } catch (err: any) {
      toast('生成 Token 失败: ' + (err.message || '未知错误'), 'error');
    }
  };

  if (!open) return null;

  return (
    <Modal title="部署节点" onClose={onClose} wide>
      <div className="space-y-4">
        <p className="text-xs text-a-muted">
          在一台新机器上部署 Aegis 节点代理。需要目标机器的 SSH 凭据和 Join Token。
        </p>

        {/* Target IP */}
        <div>
          <label className="block text-xs font-mono text-a-fg2 mb-1">目标 IP *</label>
          <input
            type="text"
            value={targetIp}
            onChange={(e) => setTargetIp(e.target.value)}
            placeholder="例: <SERVER_B_IP>"
            className="w-full px-3 py-2 rounded-a-md bg-a-bg border border-a-border text-a-fg text-sm font-mono focus:outline-none focus:border-a-accent"
          />
        </div>

        {/* SSH User + Password */}
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block text-xs font-mono text-a-fg2 mb-1">SSH 用户</label>
            <input
              type="text"
              value={sshUser}
              onChange={(e) => setSshUser(e.target.value)}
              placeholder="root"
              className="w-full px-3 py-2 rounded-a-md bg-a-bg border border-a-border text-a-fg text-sm font-mono focus:outline-none focus:border-a-accent"
            />
          </div>
          <div>
            <label className="block text-xs font-mono text-a-fg2 mb-1">SSH 密码</label>
            <input
              type="password"
              value={sshPassword}
              onChange={(e) => setSshPassword(e.target.value)}
              placeholder="SSH 密码（不保存）"
              className="w-full px-3 py-2 rounded-a-md bg-a-bg border border-a-border text-a-fg text-sm font-mono focus:outline-none focus:border-a-accent"
            />
          </div>
        </div>

        {/* Control Plane URL */}
        <div>
          <label className="block text-xs font-mono text-a-fg2 mb-1">控制平面 URL</label>
          <input
            type="text"
            value={cpUrl}
            onChange={(e) => setCpUrl(e.target.value)}
            placeholder={window.location.origin}
            className="w-full px-3 py-2 rounded-a-md bg-a-bg border border-a-border text-a-fg text-sm font-mono focus:outline-none focus:border-a-accent"
          />
        </div>

        {/* Join Token */}
        <div>
          <label className="block text-xs font-mono text-a-fg2 mb-1">Join Token *</label>
          <div className="flex gap-2">
            <input
              type="text"
              value={joinToken}
              onChange={(e) => setJoinToken(e.target.value)}
              placeholder="粘贴 Join Token 或点击生成"
              className="flex-1 px-3 py-2 rounded-a-md bg-a-bg border border-a-border text-a-fg text-sm font-mono focus:outline-none focus:border-a-accent"
            />
            <Btn onClick={handleGenerateToken}>
              生成
            </Btn>
          </div>
        </div>

        {/* Actions */}
        <div className="flex gap-3 pt-2">
          <Btn primary onClick={handleDeploy} disabled={deploying}>
            {deploying ? '部署中...' : '一键部署'}
          </Btn>
          <Btn onClick={onClose}>
            取消
          </Btn>
        </div>

        {/* Log Output */}
        {logOutput && (
          <div className="mt-3 p-3 rounded-a-md bg-a-bg border border-a-border-soft">
            <pre className="text-xs font-mono text-a-muted whitespace-pre-wrap break-all">{logOutput}</pre>
          </div>
        )}

        {/* Manual Command */}
        {manualCmd && (
          <div className="mt-3 p-3 rounded-a-md bg-a-bg border border-[#e8b830]/30">
            <p className="text-xs text-a-warn mb-2">在您的开发机上运行以下命令：</p>
            <pre className="text-xs font-mono text-a-fg whitespace-pre-wrap break-all bg-a-surface p-2 rounded">{manualCmd}</pre>
          </div>
        )}
      </div>
    </Modal>
  );
}
