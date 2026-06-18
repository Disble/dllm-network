import { NavRail } from './nav-rail';
import { TitleBar } from './title-bar';
import type { AppShellProps } from './app-shell.types';

/**
 * AppShell is the application chrome: a frameless title bar across the top, a
 * slim icon nav rail down the left, and the scrollable content area for the
 * dashboard. Pure layout — it holds no telemetry state.
 */
export function AppShell({ children }: Readonly<AppShellProps>) {
  return (
    <div className="app-shell-frame">
      <TitleBar />
      <div className="app-shell-frame__body">
        <NavRail />
        <div className="app-shell-frame__content">{children}</div>
      </div>
    </div>
  );
}
