import { Component, type ErrorInfo, type ReactNode } from 'react';

interface Props {
  children: ReactNode;
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

    return (
      <div className="min-h-screen flex items-center justify-center bg-background font-body p-8">
        <div className="w-full max-w-md text-center border border-outline-variant/30 rounded-xl p-12 bg-surface-container-high/80 ">
          <div className="w-16 h-16 rounded-full bg-error/10 border-2 border-error/40 flex items-center justify-center mx-auto mb-6">
            <span className="material-symbols-outlined text-error text-3xl">warning</span>
          </div>

          <h1 className="font-headline text-2xl font-black text-on-surface tracking-tight mb-3">
            Something went wrong
          </h1>

          <p className="text-on-surface-variant text-sm leading-relaxed mb-6">
            The application encountered an unexpected error. You can try reloading
            the page to recover.
          </p>

          {this.state.error && (
            <pre className="bg-error/5 border border-error/15 rounded-lg p-4 text-error text-xs text-left overflow-auto max-h-32 mb-8 whitespace-pre-wrap break-word">
              {this.state.error.message}
            </pre>
          )}

          <div className="flex gap-3 justify-center">
            <button
              onClick={this.handleRetry}
              className="bg-primary text-on-primary font-headline font-bold py-3 px-6 rounded-lg text-xs tracking-widest uppercase hover:brightness-110 active:scale-95 transition-[filter,transform]"
            >
              Try Again
            </button>
            <button
              onClick={this.handleReload}
              className="bg-surface-container-highest text-on-surface border border-outline-variant/30 font-headline font-bold py-3 px-6 rounded-lg text-xs tracking-widest uppercase hover:bg-surface-container-highest/80 active:scale-95 transition-[background-color,transform]"
            >
              Reload Page
            </button>
          </div>
        </div>
      </div>
    );
  }
}
