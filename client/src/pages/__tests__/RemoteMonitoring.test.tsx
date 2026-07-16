import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import RemoteMonitoring from '../RemoteMonitoring';
import * as remoteApi from '../../api/remoteApi';

vi.mock('../../hooks/useSocket', () => ({
  useSocketContext: () => ({ subscribe: vi.fn(() => () => {}) }),
}));

vi.mock('../../api/remoteApi', () => ({
  getRemoteOverview: vi.fn(),
  getRemoteInstances: vi.fn(),
  getRemoteInstance: vi.fn(),
  createRemoteInstance: vi.fn(),
  deleteRemoteInstance: vi.fn(),
  setRemoteMode: vi.fn(),
  testRemoteInstance: vi.fn(),
}));

const overview = { data: { totalInstances: 0, online: 0, offline: 0, degraded: 0, alertCount: 0 }, success: true };
type RemoteInstancesResponse = Awaited<ReturnType<typeof remoteApi.getRemoteInstances>>;

describe('RemoteMonitoring', () => {
  beforeEach(() => {
    vi.mocked(remoteApi.getRemoteOverview).mockResolvedValue(overview);
  });

  it('renders a recoverable error when the instances response is not a list', async () => {
    vi.mocked(remoteApi.getRemoteInstances).mockResolvedValue({ data: { unexpected: true }, success: true } as unknown as RemoteInstancesResponse);

    render(<RemoteMonitoring />);

    expect(await screen.findByText('Remote instances returned an unexpected response. Please try again.')).toBeInTheDocument();
    expect(screen.getByText('No remote instances yet')).toBeInTheDocument();
  });

  it('accepts legacy instance-list wrappers', async () => {
    vi.mocked(remoteApi.getRemoteInstances).mockResolvedValue({
      data: { instances: [{ id: 1, name: 'Mumbai', url: 'https://mumbai.example', tags: [], pollIntervalS: 60, tlsSkipVerify: false, status: 'online' }] },
      success: true,
    } as unknown as RemoteInstancesResponse);

    render(<RemoteMonitoring />);

    expect(await screen.findByText('Mumbai')).toBeInTheDocument();
  });
});
