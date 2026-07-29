import { Component, type ErrorInfo, type ReactNode } from 'react';

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

/**
 * ErrorBoundary keeps a render-time exception from blanking the whole console.
 *
 * WHY this exists: this UI is the only view an operator has into live gateways.
 * Without a boundary, one unexpected shape in one API response takes the entire
 * page to a white screen — and the moment that is most likely to happen is
 * during an incident, which is exactly when losing visibility costs the most.
 *
 * The fallback deliberately keeps the reload affordance and shows the message,
 * because "something broke, here is what and here is how to get back" is
 * actionable where a blank page is not.
 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // Left as console output on purpose: there is no error-reporting sink in this
    // deployment, and swallowing it would make the fallback the only evidence.
    console.error('Unhandled render error:', error, info.componentStack);
  }

  handleReload = () => {
    this.setState({ error: null });
    window.location.reload();
  };

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;

    return (
      <div
        role="alert"
        style={{
          padding: '2rem',
          maxWidth: '42rem',
          margin: '3rem auto',
          fontFamily: 'system-ui, sans-serif',
          border: '1px solid #d4534d',
          borderRadius: '6px',
          background: '#fff5f5',
          color: '#1a1a1a',
        }}
      >
        <h1 style={{ fontSize: '1.125rem', margin: '0 0 0.75rem' }}>界面出错了</h1>
        <p style={{ margin: '0 0 0.75rem', lineHeight: 1.6 }}>
          这是前端渲染异常，网关本身仍按当前配置运行，流量不受影响。
        </p>
        <pre
          style={{
            margin: '0 0 1rem',
            padding: '0.75rem',
            background: '#fff',
            border: '1px solid #e5c9c7',
            borderRadius: '4px',
            fontSize: '0.8125rem',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
          }}
        >
          {error.message || String(error)}
        </pre>
        <button
          type="button"
          onClick={this.handleReload}
          style={{
            padding: '0.5rem 1rem',
            fontSize: '0.875rem',
            cursor: 'pointer',
            borderRadius: '4px',
            border: '1px solid #999',
            background: '#fff',
          }}
        >
          重新加载
        </button>
      </div>
    );
  }
}
