import type { RunningModelView } from '../../shared/contracts/dashboard-snapshot.types';
import { formatBytes, formatExpiresAt } from '../../shared/helpers/formatters.helpers';
import type { RunningModelCardViewModel } from './running-models.types';

/** Sentinel for absent string fields. */
const ABSENT = '—';

/**
 * buildRunningModelCardViewModel maps a RunningModelView to a fully display-ready card view model.
 * Absent or zero-value fields render as "—" rather than misleading zeros.
 */
export function buildRunningModelCardViewModel(model: RunningModelView): RunningModelCardViewModel {
  return {
    name: model.name,
    parameterSize: model.parameterSize === '' ? ABSENT : model.parameterSize,
    quantizationLevel: model.quantizationLevel === '' ? ABSENT : model.quantizationLevel,
    sizeLabel: formatBytes(model.size),
    sizeVramLabel: formatBytes(model.sizeVram),
    contextLengthLabel: String(model.contextLength),
    expiresAtLabel: formatExpiresAt(model.expiresAt),
  };
}
