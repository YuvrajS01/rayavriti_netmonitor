import { memo } from 'react';

interface SkeletonProps {
  className?: string;
}

function SkeletonInner({ className = '' }: SkeletonProps) {
  return (
    <div className={`bg-surface-3 rounded animate-pulse ${className}`} />
  );
}

export const Skeleton = memo(SkeletonInner);
