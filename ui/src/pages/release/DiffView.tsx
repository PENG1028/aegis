import { useDiff } from '@/hooks/useDiff';
import { PageHeader } from '@/components/shared';
import { ReleaseDiffViewer } from '@/components/workspace/ReleaseDiffViewer';

export default function DiffView() {
  const { data: diff, isLoading } = useDiff();

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="配置差异" subtitle="Human-readable + Raw diff" />
      <ReleaseDiffViewer diff={diff || null} loading={isLoading} />
    </div>
  );
}
