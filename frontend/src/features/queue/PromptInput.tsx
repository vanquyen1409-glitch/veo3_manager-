import { useEffect, useMemo, useState } from 'react';
import { Plus } from 'lucide-react';
import { Card, CardHeader, CardTitle } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { cn } from '../../lib/cn';

interface Props {
  onEnqueue: (prompts: string[]) => void;
  disabled?: boolean;
}

type Mode = 'multi' | 'single';
const MODE_KEY = 'veo3.promptInputMode';

function loadMode(): Mode {
  try {
    const v = localStorage.getItem(MODE_KEY);
    return v === 'single' ? 'single' : 'multi';
  } catch {
    return 'multi';
  }
}

export default function PromptInput({ onEnqueue, disabled }: Props) {
  const [text, setText] = useState('');
  const [mode, setMode] = useState<Mode>(loadMode);

  useEffect(() => {
    try {
      localStorage.setItem(MODE_KEY, mode);
    } catch {
      // ignore quota / private-mode errors
    }
  }, [mode]);

  const prompts = useMemo(() => {
    if (mode === 'single') {
      const t = text.trim();
      return t ? [t] : [];
    }
    return text
      .split(/\n\s*\n+/)
      .map((p) => p.trim())
      .filter(Boolean);
  }, [text, mode]);

  function handleEnqueue() {
    if (prompts.length === 0) return;
    onEnqueue(prompts);
    setText('');
  }

  const placeholder =
    mode === 'single'
      ? 'Mô tả video bạn muốn tạo...\nMọi xuống dòng được giữ nguyên trong 1 prompt.'
      : 'Mô tả video bạn muốn tạo...\nĐể 1 dòng trống giữa các prompt khác nhau.';

  return (
    <Card className="flex h-full flex-col">
      <CardHeader>
        <CardTitle>
          Nhập prompt
          <span className="ml-2 font-normal text-muted">
            {mode === 'single' ? '(1 prompt duy nhất)' : '(ngăn cách bằng dòng trống)'}
          </span>
        </CardTitle>

        <div className="surface-elev inline-flex rounded-md border divider p-0.5 text-xs">
          {(['multi', 'single'] as const).map((m) => (
            <button
              key={m}
              type="button"
              onClick={() => setMode(m)}
              className={cn(
                'rounded px-2.5 py-1 transition',
                mode === m ? 'bg-accent-600 text-white' : 'text-soft hover-surface',
              )}
              title={
                m === 'multi'
                  ? 'Mỗi đoạn (cách nhau bằng dòng trống) là 1 prompt'
                  : 'Toàn bộ nội dung là 1 prompt duy nhất'
              }
            >
              {m === 'multi' ? 'Multi' : 'Single'}
            </button>
          ))}
        </div>
      </CardHeader>

      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder={placeholder}
        className="min-h-[260px] flex-1 resize-none font-mono text-xs"
      />

      <div className="mt-3 flex items-center justify-between">
        <span className="text-xs text-muted">{prompts.length} prompt</span>
        <Button onClick={handleEnqueue} disabled={disabled || prompts.length === 0}>
          <Plus size={14} /> Thêm vào hàng đợi
        </Button>
      </div>
    </Card>
  );
}
