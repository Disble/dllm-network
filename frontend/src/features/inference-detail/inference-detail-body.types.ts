/**
 * FormattedJsonStream is the result of pretty-printing an NDJSON body.
 */
export interface FormattedJsonStream {
  /** Number of JSON documents parsed from the stream. */
  readonly count: number;
  /** Pretty-printed JSON text, with multiple documents separated by newlines. */
  readonly pretty: string;
}
