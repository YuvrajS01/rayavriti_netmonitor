import { Component, type ErrorInfo, type ReactNode } from 'react';
import Button from './ui/Button';
import { isChunkLoadError } from '../utils/lazyWithRetry';

interface Props {
  children: ReactNode;
  /**
   * `app` (default) fills the viewport; `route` renders inside the page
   * area so navigation stays usable when a single page crashes.
   */
  scope?: 'app' | 'route';
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error('[ErrorBoundary] Uncaught error:', error);
    console.error('[ErrorBoundary] Component stack:', info.componentStack);
  }

  private handleRetry = (): void => {
    this.setState({ hasError: false, error: null });
  };

  private handleReload = (): void => {
    window.location.reload();
  };

  render(): ReactNode {
    if (!this.state.hasError) {
      return this.props.children;
    }

    const staleBuild = isChunkLoadError(this.state.error);
    const container = this.props.scope === 'route'
      ? 'min-h-[60vh] flex items-center justify-center p-8'
      : 'min-h-screen flex items-center justify-center bg-canvas p-8';

    return (
      <div className={container} role="alert">
        <div className="w-full max-w-md text-center border border-line rounded-lg p-10 bg-surface-1 shadow-e1">
          <div className="w-14 h-14 rounded-full bg-error/12 flex items-center justify-center mx-auto mb-6">
            <span className="material-symbols-outlined text-error text-3xl" aria-hidden="true">warning</span>
          </div>

          <h1 className="text-2xl font-semibold text-fg mb-3">
            {staleBuild ? 'Update available' : 'Something went wrong'}
          </h1>

          <p className="text-fg-muted text-sm leading-relaxed mb-6">
            {staleBuild
              ? 'A newer version of the application was deployed. Reload the page to continue.'
              : 'The application encountered an unexpected error. You can try reloading the page to recover.'}
          </p>

          {this.state.error && !staleBuild && (
            <pre className="bg-inset border border-line rounded-md p-4 text-fg-muted font-mono text-xs text-left overflow-auto max-h-32 mb-6 whitespace-pre-wrap break-word">
              {this.state.error.message}
            </pre>
          )}

          <div className="flex gap-3 justify-center">
            {!staleBuild && (
              <Button onClick={this.handleRetry}>Try again</Button>
            )}
            <Button variant="secondary" onClick={this.handleReload}>Reload page</Button>
          </div>
        </div>
      </div>
    );
  }
}
