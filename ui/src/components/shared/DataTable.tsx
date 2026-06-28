import { EmptyState } from './EmptyState';

export interface DataTableColumn<T> {
  key: string;
  label: string;
  render?: (row: T) => React.ReactNode;
  mono?: boolean;
  muted?: boolean;
  className?: string;
}

interface DataTableProps<T> {
  columns: DataTableColumn<T>[];
  data: T[];
  emptyMessage?: string;
  keyExtractor?: (row: T) => string;
}

export function DataTable<T extends Record<string, any>>({
  columns,
  data,
  emptyMessage,
  keyExtractor,
}: DataTableProps<T>) {
  if (!data || data.length === 0) {
    return <EmptyState title={emptyMessage || '暂无数据'} />;
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm border-collapse">
        <thead>
          <tr>
            {columns.map((col) => (
              <th
                key={col.key}
                className={`text-left px-3 py-2 text-[11px] font-semibold uppercase tracking-[0.06em] text-a-muted bg-a-bg border-b border-a-border whitespace-nowrap ${col.className || ''}`}
              >
                {col.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.map((row, i) => (
            <tr
              key={keyExtractor ? keyExtractor(row) : i}
              className="hover:bg-white/[0.03] transition-colors [&:last-child>td]:border-b-0"
            >
              {columns.map((col) => (
                <td
                  key={col.key}
                  className={`px-3 py-2.5 border-b border-a-border-soft text-xs ${
                    col.mono ? 'font-mono tabular-nums' : ''
                  } ${col.muted ? 'text-a-muted' : 'text-a-fg'} ${col.className || ''}`}
                >
                  {col.render ? col.render(row) : row[col.key]}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
