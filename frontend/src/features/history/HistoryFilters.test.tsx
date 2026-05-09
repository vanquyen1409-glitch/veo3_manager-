import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as React from 'react';
import HistoryFilters from './HistoryFilters';

describe('<HistoryFilters />', () => {
  it('renders all status options in Vietnamese', () => {
    render(<HistoryFilters status="all" search="" onStatus={vi.fn()} onSearch={vi.fn()} />);
    expect(screen.getByRole('option', { name: 'Tất cả' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Thành công' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Thất bại' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Đã hủy' })).toBeInTheDocument();
  });

  it('reflects the controlled status prop on the select', () => {
    render(<HistoryFilters status="failed" search="" onStatus={vi.fn()} onSearch={vi.fn()} />);
    const select = screen.getByRole('combobox') as HTMLSelectElement;
    expect(select.value).toBe('failed');
  });

  it('emits typed status changes through onStatus', async () => {
    const onStatus = vi.fn();
    render(<HistoryFilters status="all" search="" onStatus={onStatus} onSearch={vi.fn()} />);
    const select = screen.getByRole('combobox');
    await userEvent.selectOptions(select, 'succeeded');
    expect(onStatus).toHaveBeenCalledWith('succeeded');
  });

  it('forwards search input to onSearch on every keystroke (parent debounces)', async () => {
    // HistoryFilters is fully controlled - test via a stateful wrapper so
    // typing accumulates the way the real HistoryPage parent observes it.
    const onSearch = vi.fn<(s: string) => void>();
    function Harness() {
      const [s, setS] = React.useState('');
      return (
        <HistoryFilters
          status="all"
          search={s}
          onStatus={vi.fn()}
          onSearch={(v) => { onSearch(v); setS(v); }}
        />
      );
    }
    render(<Harness />);
    const input = screen.getByPlaceholderText('Tìm kiếm prompt...');
    await userEvent.type(input, 'hello');
    expect(onSearch).toHaveBeenCalledTimes(5);
    expect(onSearch).toHaveBeenLastCalledWith('hello');
  });

  it('reflects the controlled search prop in the input', () => {
    render(<HistoryFilters status="all" search="cat surfing" onStatus={vi.fn()} onSearch={vi.fn()} />);
    expect(screen.getByPlaceholderText('Tìm kiếm prompt...')).toHaveValue('cat surfing');
  });
});
