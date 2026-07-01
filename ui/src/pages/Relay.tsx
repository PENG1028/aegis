import { useState } from 'react';
import { PageHeader, TabBar } from '@/components/shared';
import RelayResolve from '@/components/relay/RelayResolve';
import RelayTrace from '@/components/relay/RelayTrace';

export default function RelayPage() {
  const [tab, setTab] = useState('resolve');

  return (
    <div>
      <PageHeader title="受管中继" helpKey="relay" sub="中继路径解析与运行时" />
      <TabBar
        tabs={[
          { key: 'resolve', label: '解析' },
          { key: 'trace', label: '跟踪' },
        ]}
        active={tab}
        onChange={setTab}
      />
      {tab === 'resolve' && <RelayResolve />}
      {tab === 'trace' && <RelayTrace />}
    </div>
  );
}

