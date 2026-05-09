import { Component, type ReactNode } from 'react';

interface Props { children: ReactNode }
interface State { error?: Error }

export default class ErrorBoundary extends Component<Props, State> {
  state: State = {};

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error) {
    console.error('UI crash:', error);
  }

  render() {
    if (this.state.error) {
      return (
        <div className="surface-app flex h-screen flex-col items-center justify-center gap-3 p-8 text-body">
          <h1 className="text-lg font-semibold text-red-600 dark:text-red-400">Something went wrong</h1>
          <pre className="surface-panel text-body max-w-2xl whitespace-pre-wrap rounded-md p-4 text-xs">
            {String(this.state.error?.message ?? this.state.error)}
          </pre>
          <button
            onClick={() => location.reload()}
            className="surface-elev hover-surface text-strong rounded-md px-3 py-1.5 text-sm"
          >
            Reload
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
