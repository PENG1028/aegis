// ─── ConfirmDialog Component ───
// "Type X to confirm" dialog for high-risk operations.

import { useState } from 'react';
import { Modal } from './Modal';
import { Btn } from './Btn';
import Input from '@/components/ui/Input';

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  message: string;
  confirmationText: string;
  confirmLabel?: string;
  onConfirm: () => void;
  onCancel: () => void;
  loading?: boolean;
}

export function ConfirmDialog({
  open,
  title,
  message,
  confirmationText,
  confirmLabel = '确认执行',
  onConfirm,
  onCancel,
  loading,
}: ConfirmDialogProps) {
  const [typed, setTyped] = useState('');
  const matches = typed === confirmationText;

  return (
    <Modal
      title={title}
      onClose={onCancel}
      footer={
        <div className="flex items-center gap-3">
          <Btn onClick={onCancel} className="text-xs">取消</Btn>
          <Btn
            danger
            onClick={onConfirm}
            disabled={!matches || loading}
            className="text-xs"
          >
            {loading ? '执行中...' : confirmLabel}
          </Btn>
        </div>
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-a-fg2">{message}</p>
        <div className="p-3 rounded-a-md bg-[#ff5c72]/10 border border-[#ff5c72]/20">
          <p className="text-xs text-[#ff5c72] font-medium mb-2">⚠️ 此操作不可撤销</p>
          <p className="text-xs text-a-muted">
            请输入 <code className="px-1.5 py-0.5 rounded bg-a-bg text-a-fg font-mono text-xs">{confirmationText}</code> 以确认：
          </p>
          <Input
            value={typed}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setTyped(e.target.value)}
            placeholder={confirmationText}
            className="mt-2 text-sm"
            autoFocus
          />
          {typed && !matches && (
            <p className="text-xs text-[#ff5c72] mt-1">输入不匹配</p>
          )}
        </div>
      </div>
    </Modal>
  );
}
