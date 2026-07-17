import { memo } from 'react';
function DashboardSkeletonInner() {
  return (
    <div>
      {/* Header skeleton */}
      <div className="mb-6">
        <div className="shimmer h-10 w-64 mb-2 rounded" />
        <div className="shimmer h-4 w-96 max-w-full rounded" />
      </div>

      {/* Stats grid */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="bg-surface-container-low rounded-lg p-5 border border-outline-variant/20">
            <div className="shimmer h-3 w-20 mb-3 rounded" /><div className="shimmer h-8 w-16 mb-3 rounded" /><div className="shimmer h-5 w-full rounded" />
          </div>
        ))}
      </div>

      {/* AI Health + Insights */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6">
        <div className="bg-surface-container-low rounded-lg p-5 border border-outline-variant/20 flex flex-col items-center">
          <div className="shimmer h-3 w-24 mb-4 rounded" /><div className="shimmer w-[120px] h-[120px] rounded-full" /><div className="shimmer h-3 w-32 mt-4 rounded" />
        </div>
        <div className="xl:col-span-2 bg-surface-container-low rounded-lg p-5 border border-outline-variant/20">
          <div className="shimmer h-4 w-32 mb-4 rounded" />
          <div className="grid grid-cols-2 gap-3">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="rounded-lg border border-outline-variant/20 p-3">
                <div className="shimmer h-3 w-20 mb-2 rounded" /><div className="shimmer h-3 w-full rounded" />
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6 mb-6">
        <div className="xl:col-span-2 bg-surface-container-low rounded-lg p-4 border border-outline-variant/20">
          <div className="shimmer h-4 w-40 mb-4 rounded" /><div className="shimmer h-[240px] w-full rounded" />
        </div>
        <div className="bg-surface-container-low rounded-lg p-4 border border-outline-variant/20">
          <div className="shimmer h-4 w-32 mb-4 rounded" /><div className="shimmer h-[180px] w-full rounded-full" />
        </div>
      </div>

      {/* Bottom tables */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        <div className="bg-surface-container-low rounded-lg p-6 border border-outline-variant/20">
          <div className="shimmer h-4 w-32 mb-6 rounded" />
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="flex gap-4 py-3 border-b border-outline-variant/10">
              <div className="shimmer h-4 w-24 rounded" /><div className="shimmer h-4 w-16 rounded" /><div className="shimmer h-4 w-16 rounded" /><div className="shimmer h-4 w-12 ml-auto rounded" />
            </div>
          ))}
        </div>
        <div className="bg-surface-container-low rounded-lg p-6 border border-outline-variant/20">
          <div className="shimmer h-4 w-28 mb-6 rounded" />
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="flex gap-3 p-4 rounded-lg border border-outline-variant/20 mb-3">
              <div className="shimmer h-5 w-5 rounded-full flex-shrink-0" />
              <div className="flex-1">
                <div className="shimmer h-3 w-24 mb-2 rounded" /><div className="shimmer h-3 w-full rounded" />
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export const DashboardSkeleton = memo(DashboardSkeletonInner);
