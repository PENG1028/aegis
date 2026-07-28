export interface CapabilitySemanticsView {
  state_class: 'declarative' | 'portable_asset' | 'provider_managed' | 'runtime';
  affinity: 'none' | 'executor_sticky' | 'bundle_sticky';
  migration: 're_render' | 'reload_asset' | 'recreate' | 'restart' | 'unsupported';
  coupled_with?: string[];
}

export interface CapabilityStatusView {
  key: string;
  availability: string;
  semantics: CapabilitySemanticsView;
}

export interface ProviderCapabilityView {
  id: string;
  name: string;
  status?: string;
  installed?: boolean;
  running?: boolean;
  ready?: boolean;
  capabilities?: string[];
  capability_statuses?: CapabilityStatusView[];
}

// The declared capability list says what an executor can theoretically do.
// UI actions require the live capability instance to be ready as well.
export function capabilityIsReady(
  providers: ProviderCapabilityView[],
  capability: string,
  executorIDs?: string[],
): boolean {
  const allowed = executorIDs?.length ? new Set(executorIDs) : null;
  return providers.some(provider => {
    if (allowed && !allowed.has(provider.id)) return false;
    if (!provider.capabilities?.includes(capability)) return false;
    const instance = provider.capability_statuses?.find(item => item.key === capability);
    if (instance) return instance.availability === 'ready';
    return provider.installed === true && provider.running === true && provider.status !== 'degraded';
  });
}

export function capabilityExecutor(
  providers: ProviderCapabilityView[],
  capability: string,
  executorID: string,
): ProviderCapabilityView | undefined {
  return providers.find(provider => provider.id === executorID
    && capabilityIsReady([provider], capability));
}
