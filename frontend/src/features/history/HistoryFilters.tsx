import { Search } from 'lucide-react';
import type { ChangeEvent } from 'react';

const statuses = ['all', 'succeeded', 'failed', 'cancelled'] as const;
export type StatusFilter = typeof statuses[number];

const statusLabel: Record<StatusFilter, string> = {
  all: 'Tất cả',
  succeeded: 'Thành công',
  failed: 'Thất bại',
  cancelled: 'Đã hủy',
};

interface Props {
  status: StatusFilter;
  search: string;
  onStatus: (s: StatusFilter) => void;
  onSearch: (s: string) => void;
}

export default function HistoryFilters({ status, search, onStatus, onSearch }: Props) {
  return (
    <div className="flex flex-wrap items-center gap-3">
      <div className="surface-elev flex min-w-[16rem] flex-1 items-center gap-2 rounded-md border divider px-3">
        <Search size={14} className="text-subtle" />
        <input
          value={search}
          onChange={(e: ChangeEvent<HTMLInputElement>) => onSearch(e.target.value)}
          placeholder="Tìm kiếm prompt..."
          className="w-full border-0 bg-transparent px-0 py-2 text-sm"
        />
      </div>
      <select
        value={status}
        onChange={(e) => onStatus(e.target.value as StatusFilter)}
        className="min-w-[10rem]"
      >
        {statuses.map((s) => (
          <option key={s} value={s}>{statusLabel[s]}</option>
        ))}
      </select>
    </div>
  );
}
