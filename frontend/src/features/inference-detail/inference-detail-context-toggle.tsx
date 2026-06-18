import type { InferenceDetailContextToggleProps } from './inference-detail.types';

/**
 * InferenceDetailContextToggle is the disclosure control for the Generation
 * tab's context tokens: a count label plus a centered SVG chevron that rotates
 * when expanded. Replaces the raw unicode glyph with a crisp, aligned icon.
 * Presentational — the parent owns the open state.
 */
export function InferenceDetailContextToggle({ label, open, onToggle }: Readonly<InferenceDetailContextToggleProps>) {
  return (
    <button
      type="button"
      className="inference-detail__context-toggle"
      aria-expanded={open}
      onClick={onToggle}
    >
      <span className="inference-detail__context-toggle-label">{label}</span>
      <svg
        className={`inference-detail__context-chevron${open ? ' inference-detail__context-chevron--open' : ''}`}
        viewBox="0 0 12 12"
        width="12"
        height="12"
        aria-hidden="true"
      >
        <path d="M3 4.5 L6 7.5 L9 4.5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    </button>
  );
}
