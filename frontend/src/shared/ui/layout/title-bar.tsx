/**
 * TitleBar is the frameless-window title bar: brand identity on the left and
 * custom window controls on the right. The bar is a drag region; controls call
 * the Wails window runtime when present and no-op in a plain browser / tests.
 */
export function TitleBar() {
  const runtime = (window as unknown as {
    runtime?: {
      WindowMinimise?: () => void;
      WindowToggleMaximise?: () => void;
      WindowHide?: () => void;
    };
  }).runtime;

  return (
    <header className="app-titlebar">
      <div className="app-titlebar__brand">
        <svg className="app-titlebar__logo" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
          <path d="M9 10h.01" />
          <path d="M15 10h.01" />
          <path d="M12 2a8 8 0 0 0-8 8v12l3-3 2.5 2.5L12 19l2.5 2.5L17 19l3 3V10a8 8 0 0 0-8-8z" />
        </svg>
        <span className="app-titlebar__title">dllm-network</span>
      </div>
      <div className="app-titlebar__controls">
        <button type="button" className="app-titlebar__control" aria-label="Minimise" onClick={() => runtime?.WindowMinimise?.()}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true"><line x1="5" y1="12" x2="19" y2="12" /></svg>
        </button>
        <button type="button" className="app-titlebar__control" aria-label="Maximise" onClick={() => runtime?.WindowToggleMaximise?.()}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true"><rect x="6" y="6" width="12" height="12" rx="1" /></svg>
        </button>
        <button type="button" className="app-titlebar__control app-titlebar__control--close" aria-label="Close" onClick={() => runtime?.WindowHide?.()}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true"><line x1="6" y1="6" x2="18" y2="18" /><line x1="18" y1="6" x2="6" y2="18" /></svg>
        </button>
      </div>
    </header>
  );
}
