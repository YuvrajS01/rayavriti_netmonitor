import { memo } from 'react';

interface SkeletonProps {
  className?: string;
}

function SkeletonInner({ className = '' }: SkeletonProps) {
  return (
    <div className={`bg-surface-container-highest rounded animate-pulse ${className}`} />
  );
}

export const Skeleton = memo(SkeletonInner);
