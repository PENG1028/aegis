// ─── Entry Point Detail ───
import { useParams } from 'react-router-dom';
import { useChain } from '@/hooks/useChain';
import { PathRibbon } from '@/components/workspace/PathRibbon';
import { RelationshipMap } from '@/components/workspace/RelationshipMap';
import { PageHeader, LoadingState, ErrorBanner } from '@/components/shared';

export default function EntryPointDetail() {
  const { entryId } = useParams<{ entryId: string }>();
  const { data: chain, isLoading, error } = useChain('route', entryId);

  if (isLoading) return <div className="p-6"><LoadingState text="加载链路..." /></div>;
  if (error) return <div className="p-6"><ErrorBanner message={(error as Error).message} /></div>;
  if (!chain) return <div className="p-6"><ErrorBanner message="未找到入口点" /></div>;

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title={chain.entryPoint?.domain || '入口点详情'}
        subtitle="完整链路视图"
      />
      <PathRibbon chain={chain} focusType="entry" focusId={entryId} />
      <RelationshipMap chain={chain} focusType="entry" focusId={entryId || ''} />
    </div>
  );
}
