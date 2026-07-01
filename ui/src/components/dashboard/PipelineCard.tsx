import { useNavigate } from 'react-router-dom';
import { Card } from '@/components/shared';

export interface PipelineStep {
  key: string;
  label: string;
  desc: string;
  stat?: string;
  link: string;
  status: 'ok' | 'err' | 'warn' | 'idle';
}

export default function PipelineCard({ steps }: { steps: PipelineStep[] }) {
  const navigate = useNavigate();
  return (
    <Card title="部署流水线">
      <div className="flex flex-wrap gap-0">
        {steps.map((s, i) => (
          <button
            key={s.key}
            onClick={() => navigate(s.link)}
            className="flex-1 min-w-[100px] flex flex-col items-center gap-1.5 py-3 px-2 text-center border-none bg-transparent cursor-pointer hover:bg-a-border-soft/30 transition-colors group relative"
          >
            <span className={`w-6 h-6 rounded-full flex items-center justify-center text-[10px] font-bold ${
              s.status === 'ok' ? 'bg-[#4cd964]/20 text-[#4cd964]' :
              s.status === 'err' ? 'bg-[#ff5c72]/20 text-[#ff5c72]' :
              s.status === 'warn' ? 'bg-[#e8b830]/20 text-[#e8b830]' :
              'bg-a-border/30 text-a-muted'
            }`}>
              {s.status === 'ok' ? '✓' : s.status === 'err' ? '✗' : s.status === 'warn' ? '!' : i + 1}
            </span>
            <span className="text-[11px] font-medium text-a-fg group-hover:text-a-accent transition-colors">{s.label}</span>
            <span className="text-[10px] text-a-muted leading-tight">{s.desc}</span>
            {s.stat && <span className="text-[10px] font-mono text-a-accent">{s.stat}</span>}
          </button>
        ))}
      </div>
    </Card>
  );
}