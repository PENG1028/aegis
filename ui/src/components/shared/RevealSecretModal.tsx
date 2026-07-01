import { useState } from 'react';

interface RevealSecretModalProps {
  title: string;
  secret: string;
  warning?: string;
  onClose: () => void;
}

export default function RevealSecretModal({ title, secret, warning, onClose }: RevealSecretModalProps) {
  const [copied, setCopied] = useState(false);

  async function doCopy() {
    await navigator.clipboard.writeText(secret);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={onClose}>
      <div
        className="bg-a-surface border border-a-border rounded-a-lg p-6 w-full max-w-lg shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <h3 className="text-sm font-semibold mb-3">{title}</h3>

        {warning && (
          <div className="mb-3 p-2 rounded-a-sm text-xs bg-[#e8b830]/10 text-[#e8b830] border border-[#e8b830]/20">
            {warning}
          </div>
        )}

        <div className="bg-a-bg border border-a-border rounded-a-sm p-3 mb-3">
          <code className="text-xs font-mono break-all text-a-accent">{secret}</code>
        </div>

        <div className="flex gap-2 justify-end">
          <button
            onClick={doCopy}
            className="px-3 py-1.5 rounded-a-sm text-xs bg-a-accent/10 text-a-accent hover:bg-a-accent/20 border-none cursor-pointer transition-colors"
          >
            {copied ? '已复制 ✓' : '复制'}
          </button>
          <button
            onClick={onClose}
            className="px-3 py-1.5 rounded-a-sm text-xs bg-a-border/20 text-a-muted hover:bg-a-border/30 border-none cursor-pointer transition-colors"
          >
            关闭
          </button>
        </div>
      </div>
    </div>
  );
}
