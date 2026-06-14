package dashboard

import "context"

const TopicDashboardSnapshot = "dashboard:snapshot"

type Emitter interface {
	Emit(context.Context, string, any) error
}

type Publisher struct {
	projector *Projector
	recent    RecentReader
	emitter   Emitter
}

func NewPublisher(projector *Projector, recent RecentReader, emitter Emitter) *Publisher {
	if projector == nil {
		projector = NewProjector(nil)
	}

	return &Publisher{
		projector: projector,
		recent:    recent,
		emitter:   emitter,
	}
}

func (publisher *Publisher) Publish(ctx context.Context, input ProjectionInput) (Snapshot, error) {
	snapshot := publisher.projector.Project(input, publisher.recent)
	if publisher.emitter == nil {
		return snapshot, nil
	}

	if err := publisher.emitter.Emit(ctx, TopicDashboardSnapshot, snapshot); err != nil {
		return Snapshot{}, err
	}

	return snapshot, nil
}
