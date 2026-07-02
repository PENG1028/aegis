import { useParams } from 'react-router-dom';
import { useChain } from '@/hooks/useChain';
import { PathRibbon } from '@/components/workspace/PathRibbon';
import { RelationshipMap } from '@/components/workspace/RelationshipMap';
import { PageHeader, LoadingState, ErrorBanner } from '@/components/shared';

export default function GatewayDetail() {
  const { gatewayId } = useParams<{ gatewayId: string }>();
  const { data: chain, isLoading, error } = useChain('gateway', gatewayId);

  if (isLoading) return <div className="p-6"><LoadingState text="加载网关链路..." /></div>;
  if (error) return <div className="p-6"><ErrorBanner message={(error as Error).message} /></div>;
  if (!chain?.gateway) return <div className="p-6"><ErrorBanner message={`未找到网关${chain?.error ? '：' + chain.error : ''}`} /></div>;

  return (
    <div className="p-6 space-y-6">
      <PageHeader title={chain.gateway.name} subtitle={`${chain.gateway.provider} · ${chain.gateway.host}:${chain.gateway.port}`} />
      <PathRibbon chain={chain} focusType="gateway" focusId={gatewayId} />
      <RelationshipMap chain={chain} focusType="gateway" focusId={gatewayId || ''} />
    </div>
  );
}
