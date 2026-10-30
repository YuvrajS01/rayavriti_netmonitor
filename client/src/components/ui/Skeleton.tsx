import { memo } from 'react';

interface SkeletonProps {
  className?: string;
}

function SkeletonInner({ className = '' }: SkeletonProps) {
  return (
    <div aria-hidden="true" className={`shimmer rounded-md ${className}`} />
  );
}

export const Skeleton = memo(SkeletonInner);
