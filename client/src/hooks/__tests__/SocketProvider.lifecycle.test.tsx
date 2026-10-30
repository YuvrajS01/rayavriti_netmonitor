import { act, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SocketProvider } from '../SocketProvider';
import { useSocketConnected } from '../socketContext';
import { useSocketEvents, type SocketHandlers } from '../useSocketEvents';
import { BATCH_WINDOW_MS, reconnectDelay } from '../socketBackoff';
import { sessionStore } from '../../store/session';
import { getFreshAccessToken } from '../../api/http';

vi.mock('../../api/http', () => ({
  getFreshAccessToken: vi.fn(() => Promise.resolve<string | null>('access-token')),
}));

class MockWebSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  static instances: MockWebSocket[] = [];

  readyState = MockWebSocket.CONNECTING;
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onclose: ((event: { code: number }) => void) | null = null;
  onerror: (() => void) | null = null;
  sent: string[] = [];
  readonly url: string;
  readonly protocols?: string[];

  constructor(url: string, protocols?: string[]) {
    this.url = url;
    this.protocols = protocols;
    MockWebSocket.instances.push(this);
  }

  send(data: string) { this.sent.push(data); }

  close(code = 1000) {
    if (this.readyState === MockWebSocket.CLOSED) return;
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.({ code });
  }

  // Test helpers
  open() {
    this.readyState = MockWebSocket.OPEN;
    this.onopen?.();
  }

  receive(type: string, data: unknown) {
    this.onmessage?.({ data: JSON.stringify({ type, data }) });
  }

  drop() {
    this.close(1006);
  }
}

const latest = () => MockWebSocket.instances[MockWebSocket.instances.length - 1]!;

/** Flush the async token lookup that precedes each connect. */
const flush = () => act(async () => {});
const advance = (ms: number) => act(async () => { await vi.advanceTimersByTimeAsync(ms); });

function Status() {
  return <span data-testid="status">{useSocketConnected() ? 'live' : 'down'}</span>;
}

function Listener({ handlers }: { handlers: SocketHandlers }) {
  useSocketEvents(handlers);
  return null;
}

async function renderProvider(handlers: SocketHandlers = {}) {
  const view = render(
    <SocketProvider>
      <Status />
      <Listener handlers={handlers} />
    </SocketProvider>,
  );
  await flush();
  return view;
}

describe('SocketProvider lifecycle', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    MockWebSocket.instances = [];
    vi.stubGlobal('WebSocket', MockWebSocket);
    vi.spyOn(Math, 'random').mockReturnValue(0);
    sessionStore.setAuthenticated({ id: 1, username: 'admin', role: 'admin' });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    act(() => sessionStore.setUnauthenticated());
  });

  it('connects on mount with a fresh token and reports connection state', async () => {
    await renderProvider();
    expect(MockWebSocket.instances).toHaveLength(1);
    expect(latest().protocols).toEqual(['access-token']);
    expect(screen.getByTestId('status')).toHaveTextContent('down');
    act(() => latest().open());
    expect(screen.getByTestId('status')).toHaveTextContent('live');
  });

  it('falls back to the cookie when no token is available', async () => {
    vi.mocked(getFreshAccessToken).mockResolvedValueOnce(null);
    await renderProvider();
    expect(latest().protocols).toBeUndefined();
  });

  it('delivers typed events to subscribers', async () => {
    const onDeviceStatus = vi.fn();
    await renderProvider({ onDeviceStatus });
    act(() => latest().open());
    act(() => latest().receive('device:status', { deviceId: 7, status: 'down' }));
    expect(onDeviceStatus).toHaveBeenCalledWith({ deviceId: 7, status: 'down' });
  });

  it('ignores unknown event types and malformed frames', async () => {
    const onDeviceStatus = vi.fn();
    await renderProvider({ onDeviceStatus });
    act(() => latest().open());
    act(() => {
      latest().receive('packet:captured', {});
      latest().onmessage?.({ data: 'not json' });
    });
    expect(onDeviceStatus).not.toHaveBeenCalled();
  });

  it('reconnects with exponential backoff after an unexpected close', async () => {
    await renderProvider();
    act(() => latest().drop());
    expect(screen.getByTestId('status')).toHaveTextContent('down');
    expect(MockWebSocket.instances).toHaveLength(1);

    // attempt 0 → 500ms with random() = 0
    await advance(499);
    expect(MockWebSocket.instances).toHaveLength(1);
    await advance(1);
    expect(MockWebSocket.instances).toHaveLength(2);

    // attempt 1 → 1000ms
    act(() => latest().drop());
    await advance(999);
    expect(MockWebSocket.instances).toHaveLength(2);
    await advance(1);
    expect(MockWebSocket.instances).toHaveLength(3);

    // A successful open resets the backoff.
    act(() => latest().open());
    act(() => latest().drop());
    await advance(500);
    expect(MockWebSocket.instances).toHaveLength(4);
  });

  it('ignores close events from a superseded socket', async () => {
    await renderProvider();
    const first = latest();
    act(() => first.drop());
    await advance(500);
    const second = latest();
    act(() => second.open());
    expect(screen.getByTestId('status')).toHaveTextContent('live');

    // A late close from the old socket must not disconnect or respawn.
    act(() => first.onclose?.({ code: 1006 }));
    await advance(60_000);
    expect(screen.getByTestId('status')).toHaveTextContent('live');
    expect(MockWebSocket.instances).toHaveLength(2);
  });

  it('reconnects immediately when the browser comes back online', async () => {
    await renderProvider();
    act(() => latest().drop());
    expect(MockWebSocket.instances).toHaveLength(1);
    act(() => { window.dispatchEvent(new Event('online')); });
    await flush();
    expect(MockWebSocket.instances).toHaveLength(2);
    // The pending backoff timer was cancelled.
    await advance(60_000);
    expect(MockWebSocket.instances).toHaveLength(2);
  });

  it('reconnects immediately when the tab becomes visible', async () => {
    await renderProvider();
    act(() => latest().drop());
    const visibility = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
    act(() => { document.dispatchEvent(new Event('visibilitychange')); });
    await flush();
    expect(MockWebSocket.instances).toHaveLength(2);
    visibility.mockRestore();
  });

  it('does not open a second socket while one is live', async () => {
    await renderProvider();
    act(() => latest().open());
    act(() => { window.dispatchEvent(new Event('online')); });
    await flush();
    expect(MockWebSocket.instances).toHaveLength(1);
  });

  it('batches metric:update events into one delivery window', async () => {
    const onMetricUpdate = vi.fn();
    const onDeviceStatus = vi.fn();
    await renderProvider({ onMetricUpdate, onDeviceStatus });
    act(() => latest().open());
    act(() => {
      latest().receive('metric:update', { deviceId: 1 });
      latest().receive('metric:update', { deviceId: 2 });
    });
    expect(onMetricUpdate).not.toHaveBeenCalled();
    await advance(BATCH_WINDOW_MS);
    expect(onMetricUpdate).toHaveBeenCalledTimes(2);
    expect(onMetricUpdate.mock.calls.map(([m]) => (m as { deviceId: number }).deviceId)).toEqual([1, 2]);

    // A non-batched event flushes pending metrics first to keep ordering.
    const order: string[] = [];
    onMetricUpdate.mockImplementation(() => order.push('metric'));
    onDeviceStatus.mockImplementation(() => order.push('status'));
    act(() => {
      latest().receive('metric:update', { deviceId: 3 });
      latest().receive('device:status', { deviceId: 3, status: 'up' });
    });
    expect(order).toEqual(['metric', 'status']);
  });

  it('closes the socket on unmount and never reconnects', async () => {
    const view = await renderProvider();
    act(() => latest().open());
    const socket = latest();
    view.unmount();
    expect(socket.readyState).toBe(MockWebSocket.CLOSED);
    await advance(60_000);
    expect(MockWebSocket.instances).toHaveLength(1);
  });

  it('closes the socket on logout and does not connect while unauthenticated', async () => {
    await renderProvider();
    act(() => latest().open());
    const socket = latest();
    act(() => sessionStore.setUnauthenticated());
    expect(socket.readyState).toBe(MockWebSocket.CLOSED);
    expect(screen.getByTestId('status')).toHaveTextContent('down');
    await advance(60_000);
    expect(MockWebSocket.instances).toHaveLength(1);
  });
});

describe('reconnectDelay', () => {
  it('grows exponentially, caps at 30s and stays within the jitter band', () => {
    expect(reconnectDelay(0, () => 0)).toBe(500);
    expect(reconnectDelay(0, () => 1)).toBe(1000);
    expect(reconnectDelay(3, () => 0)).toBe(4000);
    expect(reconnectDelay(20, () => 0)).toBe(15000);
    expect(reconnectDelay(20, () => 1)).toBe(30000);
  });
});
