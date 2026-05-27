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

  private handleReload = (): void => {
    window.location.reload();
  };

  render(): ReactNode {
    if (!this.state.hasError) {
      return this.props.children;
    }

    return (
      <div
        style={{
          minHeight: '100vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: '#0a0a0f',
          fontFamily: "'Space Grotesk', sans-serif",
          padding: '2rem',
        }}
      >
        <div
          style={{
            maxWidth: '480px',
            width: '100%',
            textAlign: 'center',
            border: '1px solid rgba(139, 92, 246, 0.3)',
            borderRadius: '16px',
            padding: '3rem 2rem',
            background: 'rgba(15, 15, 25, 0.95)',
            boxShadow: '0 0 60px rgba(139, 92, 246, 0.08)',
          }}
        >
          {/* Icon */}
          <div
            style={{
              width: '64px',
              height: '64px',
              borderRadius: '50%',
              background: 'rgba(239, 68, 68, 0.1)',
              border: '2px solid rgba(239, 68, 68, 0.4)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              margin: '0 auto 1.5rem',
              fontSize: '28px',
            }}
          >
            ⚠
          </div>

          <h1
            style={{
              fontFamily: "'League Spartan', sans-serif",
              fontSize: '1.75rem',
              fontWeight: 900,
              color: '#f1f5f9',
              margin: '0 0 0.75rem',
              letterSpacing: '-0.02em',
            }}
          >
            Something went wrong
          </h1>

          <p
            style={{
              color: '#94a3b8',
              fontSize: '0.95rem',
              lineHeight: 1.6,
              margin: '0 0 1.5rem',
            }}
          >
            The application encountered an unexpected error. You can try reloading
            the page to recover.
          </p>

          {/* Error details */}
          {this.state.error && (
            <pre
              style={{
                background: 'rgba(239, 68, 68, 0.06)',
                border: '1px solid rgba(239, 68, 68, 0.15)',
                borderRadius: '8px',
                padding: '1rem',
                color: '#f87171',
                fontSize: '0.8rem',
                textAlign: 'left',
                overflow: 'auto',
                maxHeight: '120px',
                margin: '0 0 2rem',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
              }}
            >
              {this.state.error.message}
            </pre>
          )}

          <button
            onClick={this.handleReload}
            style={{
              background: 'linear-gradient(135deg, #8b5cf6, #6d28d9)',
              color: '#fff',
              border: 'none',
              borderRadius: '10px',
              padding: '0.75rem 2rem',
              fontSize: '0.95rem',
              fontWeight: 600,
              fontFamily: "'Space Grotesk', sans-serif",
              cursor: 'pointer',
              transition: 'transform 0.15s, box-shadow 0.15s',
              boxShadow: '0 0 20px rgba(139, 92, 246, 0.3)',
            }}
            onMouseOver={(e) => {
              (e.target as HTMLButtonElement).style.transform = 'translateY(-1px)';
              (e.target as HTMLButtonElement).style.boxShadow =
                '0 0 30px rgba(139, 92, 246, 0.5)';
            }}
            onMouseOut={(e) => {
              (e.target as HTMLButtonElement).style.transform = 'translateY(0)';
              (e.target as HTMLButtonElement).style.boxShadow =
                '0 0 20px rgba(139, 92, 246, 0.3)';
            }}
          >
            Reload Page
          </button>
        </div>
      </div>
    );
  }
}
