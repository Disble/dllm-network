import { RunningModelCard } from './running-model-card';
import { useRunningModels } from './use-running-models';
import type { RunningModelsContainerProps } from './running-models.types';

/**
 * RunningModelsContainer is a container that subscribes to the snapshot source
 * and renders RunningModelCard for each currently loaded model.
 * Follows the container/presentational split: this component manages data; cards render it.
 */
export function RunningModelsContainer({ source }: Readonly<RunningModelsContainerProps>) {
  const models = useRunningModels(source);

  if (models.length === 0) {
    return (
      <section className="running-models" aria-label="Running models">
        <p className="running-models__empty">No running models at this time.</p>
      </section>
    );
  }

  return (
    <section className="running-models" aria-label="Running models">
      {models.map((vm) => (
        <RunningModelCard key={vm.name} viewModel={vm} />
      ))}
    </section>
  );
}
