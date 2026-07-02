// ─── useScenario Hook ───
// Mock scenario switcher for development.

import { useState, useEffect, useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { getActiveScenarioId, setScenario, getScenarioList } from '@/mocks';
import type { ScenarioId } from '@/mocks';
import { API_CONFIG } from '@/lib/api-config';

export function useScenario() {
  const queryClient = useQueryClient();
  const [id, setId] = useState<ScenarioId>(getActiveScenarioId());
  const available = API_CONFIG.useMock;

  useEffect(() => {
    const handler = (e: Event) => {
      setId((e as CustomEvent).detail);
    };
    window.addEventListener('scenario-change', handler);
    return () => window.removeEventListener('scenario-change', handler);
  }, []);

  const change = useCallback((newId: ScenarioId) => {
    setScenario(newId);
    setId(newId);
    // Invalidate ALL React Query caches so pages refetch scenario data
    queryClient.invalidateQueries();
  }, [queryClient]);

  return {
    id,
    change,
    available,
    scenarios: available ? getScenarioList() : [],
  };
}
