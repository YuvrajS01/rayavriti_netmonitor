import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { resolveWebSocketUrl } from '../socketUrl';

const originalLocation = window.location;

function setLocation(url: string) {
  Object.defineProperty(window, 'location', {
    value: new URL(url),
    configurable: true,
  });
}

describe('resolveWebSocketUrl', () => {
  beforeEach(() => {
    setLocation('http://localhost:5173/dashboard');
  });

  afterEach(() => {
    Object.defineProperty(window, 'location', {
      value: originalLocation,
      configurable: true,
    });
    vi.unstubAllEnvs();
  });

  it('uses the proxied API WebSocket path by default', () => {
    expect(resolveWebSocketUrl()).toBe('ws://localhost:5173/api/v1/ws');
  });

  it('resolves same-origin paths against the current host', () => {
    expect(resolveWebSocketUrl('/api/v1/ws')).toBe('ws://localhost:5173/api/v1/ws');
  });

  it('converts HTTP URLs to WebSocket URLs', () => {
    expect(resolveWebSocketUrl('http://localhost:3000/api/v1/ws')).toBe('ws://localhost:3000/api/v1/ws');
  });

  it('uses the default path for host-only configuration values', () => {
    expect(resolveWebSocketUrl('localhost:3000')).toBe('ws://localhost:3000/api/v1/ws');
  });
});
