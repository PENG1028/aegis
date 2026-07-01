import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { fetchDashboard, dnsApi, clusterHealthApi, systemHealthApi, system } from '@/lib/api-bridge';
import { StatCard, Card, StatusBadge, Alert } from '@/components/shared';

import PipelineCard, { type PipelineStep } from '@/components/dashboard/PipelineCard';
