interface LoadingStateProps {
  text?: string;
}

export default function LoadingState({ text = '加载中...' }: LoadingStateProps) {
  return (
    <div className="text-center py-10 text-a-muted font-mono text-sm">{text}</div>
  );
}
