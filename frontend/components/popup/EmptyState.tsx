import type { ReactNode } from 'react';

export interface EmptyStateProps {
  title: string;
  text: string;
  children?: ReactNode;
}

export function EmptyState({ title, text, children }: EmptyStateProps) {
  return (
    <div className="nc-empty">
      <p className="nc-empty-title">{title}</p>
      <p className="nc-empty-text nc-small">{text}</p>
      {children}
    </div>
  );
}
