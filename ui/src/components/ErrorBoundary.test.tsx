import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ErrorBoundary } from './ErrorBoundary';

function Boom(): JSX.Element {
  throw new Error('cannot read properties of undefined');
}

describe('ErrorBoundary', () => {
  let consoleError: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    // React logs the caught error itself; silence it so a passing test is quiet.
    consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    consoleError.mockRestore();
  });

  it('renders children when nothing throws', () => {
    render(
      <ErrorBoundary>
        <div>console content</div>
      </ErrorBoundary>,
    );
    expect(screen.getByText('console content')).toBeInTheDocument();
  });

  it('replaces a thrown subtree with a fallback instead of blanking the page', () => {
    render(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>,
    );

    // The operator must still see something actionable, not an empty document.
    const alert = screen.getByRole('alert');
    expect(alert).toBeInTheDocument();
    expect(alert.textContent).toBeTruthy();
  });

  it('states that traffic is unaffected, so the operator does not escalate a UI bug', () => {
    render(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>,
    );
    // WHY: a blank console during an incident reads as "the gateway is down".
    // The fallback has to separate "the UI broke" from "traffic is broken".
    expect(screen.getByRole('alert').textContent).toMatch(/流量不受影响/);
  });

  it('surfaces the underlying message so the failure is diagnosable', () => {
    render(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>,
    );
    expect(screen.getByText(/cannot read properties of undefined/)).toBeInTheDocument();
  });

  it('offers a way back', () => {
    render(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>,
    );
    expect(screen.getByRole('button', { name: /重新加载/ })).toBeInTheDocument();
  });
});
