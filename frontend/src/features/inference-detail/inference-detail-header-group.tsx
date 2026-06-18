import type { InferenceDetailHeaderGroupProps } from './inference-detail.types';

/**
 * InferenceDetailHeaderGroup renders one labelled, ordered header table. Renders
 * nothing when the group is empty — the Headers tab owns the not-captured state.
 */
export function InferenceDetailHeaderGroup({ title, headers }: Readonly<InferenceDetailHeaderGroupProps>) {
  if (headers.length === 0) {
    return null;
  }

  return (
    <div className="inference-detail__header-group">
      <p className="section-label">{title}</p>
      <dl className="inference-detail__fields">
        {headers.map((header, index) => (
          <div key={`${header.name}::${index}`} className="inference-detail__header-row">
            <dt>{header.name}</dt>
            <dd>{header.value}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
