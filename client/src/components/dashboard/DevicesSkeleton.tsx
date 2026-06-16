import { memo } from 'react';
import { Skeleton } from '../ui/Skeleton';

function DevicesSkeletonInner() {
  return (
    <div>
      {/* Header */}
      <div className="mb-8">
        <Skeleton className="h-10 w-48 mb-2" />
        <Skeleton className="h-4 w-80" />
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-12">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20">
            <Skeleton className="h-3 w-24 mb-3" />
            <Skeleton className="h-8 w-12" />
          </div>
        ))}
      </div>

      {/* Filters */}
      <div className="flex gap-4 mb-6">
        <Skeleton className="h-10 flex-1 min-w-48" />
        <Skeleton className="h-10 w-32" />
      </div>

      {/* Device cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="bg-surface-container-low rounded-xl border border-outline-variant/20 overflow-hidden">
            <div className="p-6">
              <div className="flex justify-between items-start mb-6">
                <Skeleton className="w-12 h-12 rounded-lg" />
                <Skeleton className="h-6 w-16 rounded-full" />
              </div>
              <Skeleton className="h-6 w-32 mb-1" />
              <Skeleton className="h-4 w-40 mb-4" />
              <div className="space-y-2">
                <Skeleton className="h-3 w-full" />
                <Skeleton className="h-3 w-full" />
                <Skeleton className="h-3 w-2/3" />
              </div>
            </div>
            <div className="bg-surface-container-high p-4 flex justify-between items-center">
              <Skeleton className="h-3 w-20" />
              <Skeleton className="h-6 w-14" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export const DevicesSkeleton = memo(DevicesSkeletonInner);
