import type { ReactNode } from 'react';

/**
 * AppShellProps is the boundary for the application shell layout.
 */
export interface AppShellProps {
  /** Main content rendered inside the shell's scrollable content area. */
  readonly children: ReactNode;
}
