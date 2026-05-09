import { useMemo, useState } from 'react';
import { Plus } from 'lucide-react';
import { Card, CardHeader, CardTitle } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';

interface Props {
  onEnqueue: (prompts: string[]) => void;
  disabled?: boolean;
}

export default function PromptInput({ onEnqueue, disabled }: Props) {
  const [text, setText] = useState('');
  const lines = useMemo(
    () => text.split('\n').map((l) => l.trim()).filter(Boolean),
    [text],
  );

  function handleEnqueue() {
    if (lines.length === 0) return;
    onEnqueue(lines);
    setText('');
  }

  return (
    <Card className="flex h-full flex-col">
      <CardHeader>
        <CardTitle>Nhập prompt (mỗi dòng 1 prompt)</CardTitle>
      </CardHeader>

      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder="Mô tả video bạn muốn tạo...&#10;Mỗi dòng là 1 prompt riêng biệt"
        className="min-h-[260px] flex-1 resize-none font-mono text-xs"
      />

      <div className="mt-3 flex items-center justify-between">
        <span className="text-xs text-muted">{lines.length} prompt</span>
        <Button onClick={handleEnqueue} disabled={disabled || lines.length === 0}>
          <Plus size={14} /> Thêm vào hàng đợi
        </Button>
      </div>
    </Card>
  );
}
