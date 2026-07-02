import { useParams } from 'react-router-dom';
import { useChain } from '@/hooks/useChain';
import { PathRibbon } from '@/components/workspace/PathRibbon';
import { RelationshipMap } from '@/components/workspace/RelationshipMap';
import { PageHeader, LoadingState, ErrorBanner } from '@/components/shared';

export default function ServiceDetail() {
  const { serviceId } = useParams<{ serviceId: string }>();
  const { data: chain, isLoading, error } = useChain('service', serviceId);

  if (isLoading) return <div className="p-6"><LoadingState text="加载服务链路..." /></div>;
  if (error) return <div className="p-6"><ErrorBanner message={(error as Error).message} /></div>;
  if (!chain?.service) return <div className="p-6"><ErrorBanner message="未找到服务" /></div>;

  return (
    <div className="p-6 space-y-6">
      <PageHeader title={chain.service.name} subtitle={`${chain.endpoints.length} 个端点 · ${chain.service.latency_ms}ms`} />
      <PathRibbon chain={chain} focusType="service" focusId={serviceId} />
      <RelationshipMap chain={chain} focusType="service" focusId={serviceId || ''} />
    </div>
  );
}
