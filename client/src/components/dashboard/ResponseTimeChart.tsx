import { memo } from 'react';
import type { DeviceHistory } from '../../api/types';
import IconButton from '../ui/IconButton';
import ResponseHistoryChart from '../charts/ResponseHistoryChart';

interface Props {
  series: DeviceHistory[];
  /** `history`: bucketed server history; `live`: samples seen since page load (older servers). */
  source: 'history' | 'live';
  loading: boolean;
  onExpand: () => void;
}

function ResponseTimeChartInner({ series, source, loading, onExpand }: Props) {
  const hasData = series.some((s) => s.points.some((p) => p.samples > 0));
  return (
    <section aria-labelledby="response-time-heading" className="bg-surface-1 rounded-lg p-5 border border-line">
      <div className="flex justify-between items-start gap-3 mb-3">
        <div>
          <h3 id="response-time-heading" className="text-sm font-semibold text-fg">Response time, slowest devices</h3>
          <p className="text-xs text-fg-muted">
            {source === 'history' ? 'Last hour, 1-minute averages. Squares mark minutes with failed polls.' : 'History is unavailable on this server; showing polls received since this page opened.'}
          </p>
        </div>
        <IconButton icon="open_in_full" label="Expand response time chart" size="sm" onClick={onExpand} />
      </div>
      {loading ? (
        <div role="status" className="h-60 flex items-center justify-center text-xs text-fg-muted">Loading response times…</div>
      ) : !hasData ? (
        <p className="text-xs text-fg-muted text-center py-16">No device metrics yet</p>
      ) : (
        <ResponseHistoryChart series={series} height={240} title="Response time per device" />
      )}
    </section>
  );
}

export const ResponseTimeChart = memo(ResponseTimeChartInner);
