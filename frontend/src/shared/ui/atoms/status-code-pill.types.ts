/**
 * StatusCodePillProps is the read-only boundary for the HTTP status-code atom.
 */
export interface StatusCodePillProps {
  /**
   * HTTP response status code. null/undefined/0 render an honest em-dash —
   * passive capture may not have observed a status line yet.
   * TECH DEBT (Slice A backend): until the extractor plumbs statusCode through,
   * this is effectively always unavailable.
   */
  readonly statusCode?: number | null;
}
