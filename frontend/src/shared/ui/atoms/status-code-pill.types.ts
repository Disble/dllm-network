/**
 * StatusCodePillProps is the read-only boundary for the HTTP status-code atom.
 */
export interface StatusCodePillProps {
  /**
   * HTTP response status code. null/undefined/0 render an honest em-dash —
   * passive capture may not have observed a status line yet.
   */
  readonly statusCode?: number | null;
}
