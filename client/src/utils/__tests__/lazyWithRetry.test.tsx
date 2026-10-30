import { Suspense } from 'react';
import { render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { isChunkLoadError, lazyWithRetry } from '../lazyWithRetry';
import ErrorBoundary from '../../components/ErrorBoundary';

const chunkError = () => new TypeError('Failed to fetch dynamically imported module: /assets/Page-abc.js');
const Page = () => <p>page loaded</p>;

const originalLocation = window.location;

describe('lazyWithRetry', () => {
  let reload: ReturnType<typeof vi.fn<() => void>>;

  beforeEach(() => {
    reload = vi.fn<() => void>();
    Object.defineProperty(window, 'location', { value: { ...originalLocation, reload }, configurable: true });
    sessionStorage.clear();
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    Object.defineProperty(window, 'location', { value: originalLocation, configurable: true });
    vi.restoreAllMocks();
  });

  it('recognises chunk load failures', () => {
    expect(isChunkLoadError(chunkError())).toBe(true);
    expect(isChunkLoadError(new TypeError('Importing a module script failed.'))).toBe(true);
    expect(isChunkLoadError(new Error('boom'))).toBe(false);
    expect(isChunkLoadError('Failed to fetch dynamically imported module')).toBe(false);
  });

  it('retries a failed chunk import once', async () => {
    const factory = vi.fn<() => Promise<{ default: typeof Page }>>()
      .mockRejectedValueOnce(chunkError())
      .mockResolvedValueOnce({ default: Page });
    const Lazy = lazyWithRetry(factory);

    render(<Suspense fallback="loading"><Lazy /></Suspense>);

    expect(await screen.findByText('page loaded', {}, { timeout: 2000 })).toBeInTheDocument();
    expect(factory).toHaveBeenCalledTimes(2);
    expect(reload).not.toHaveBeenCalled();
  });

  it('reloads the page once when the retry also fails', async () => {
    const factory = vi.fn<() => Promise<{ default: typeof Page }>>().mockRejectedValue(chunkError());
    const Lazy = lazyWithRetry(factory);

    render(<Suspense fallback="loading"><Lazy /></Suspense>);

    await vi.waitFor(() => expect(reload).toHaveBeenCalledTimes(1), { timeout: 2000 });
  });

  it('surfaces the error instead of reloading in a loop', async () => {
    sessionStorage.setItem('netmonitor:chunk-reload', '1');
    const factory = vi.fn<() => Promise<{ default: typeof Page }>>().mockRejectedValue(chunkError());
    const Lazy = lazyWithRetry(factory);

    render(
      <ErrorBoundary scope="route">
        <Suspense fallback="loading"><Lazy /></Suspense>
      </ErrorBoundary>,
    );

    expect(await screen.findByText('Update available', {}, { timeout: 2000 })).toBeInTheDocument();
    expect(reload).not.toHaveBeenCalled();
  });

  it('does not retry ordinary errors', async () => {
    const factory = vi.fn<() => Promise<{ default: typeof Page }>>().mockRejectedValue(new Error('module threw'));
    const Lazy = lazyWithRetry(factory);

    render(
      <ErrorBoundary scope="route">
        <Suspense fallback="loading"><Lazy /></Suspense>
      </ErrorBoundary>,
    );

    expect(await screen.findByText('Something went wrong')).toBeInTheDocument();
    expect(factory).toHaveBeenCalledTimes(1);
  });
});

describe('route-scoped ErrorBoundary', () => {
  beforeEach(() => {
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('contains a page crash so surrounding navigation keeps rendering', () => {
    const Crash = () => { throw new Error('page exploded'); };
    render(
      <div>
        <nav>sidebar</nav>
        <ErrorBoundary scope="route"><Crash /></ErrorBoundary>
      </div>,
    );
    expect(screen.getByText('sidebar')).toBeInTheDocument();
    expect(screen.getByText('page exploded')).toBeInTheDocument();
  });
});
