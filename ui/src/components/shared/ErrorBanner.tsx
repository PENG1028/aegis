import { ReactNode } from 'react';

interface ErrorBannerProps {
  message: ReactNode;
  onRetry?: () => void;
}

export default function ErrorBanner({ message, onRetry }: ErrorBannerProps) {
  return (
    <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20 flex items-center gap-2">
      <span className="flex-1">{message}</span>
      {onRetry && (
        <button
          onClick={onRetry}
          className="px-2 py-1 rounded text-[11px] bg-[#ff5c72]/20 hover:bg-[#ff5c72]/30 text-[#ff5c72] border-none cursor-pointer transition-colors"
        >
          重试
        </button>
      )}
    </div>
  );
}
