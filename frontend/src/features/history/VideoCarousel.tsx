import { localFileURL } from '../../lib/url';

interface Props {
  paths: string[];
}

export default function VideoCarousel({ paths }: Props) {
  if (!paths || paths.length === 0) {
    return (
      <div className="text-subtle rounded-md border border-dashed divider p-4 text-center text-xs">
        No videos saved.
      </div>
    );
  }
  if (paths.length === 1) {
    return (
      <video
        src={localFileURL(paths[0])}
        controls
        preload="metadata"
        className="aspect-video w-full rounded-lg bg-black"
      />
    );
  }
  return (
    <div className="flex gap-3 overflow-x-auto pb-2 snap-x snap-mandatory">
      {paths.map((p) => (
        <video
          key={p}
          src={localFileURL(p)}
          controls
          preload="metadata"
          className="h-44 w-72 flex-none snap-start rounded-lg bg-black object-cover"
        />
      ))}
    </div>
  );
}
