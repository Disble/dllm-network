package tray

// MockTrayManager is a test double recording Start/Stop interactions.
type MockTrayManager struct {
	StartCalls  int
	StopCalls   int
	StartErr    error
	StopErr     error
	StartConfig Config
	Started     bool
	Stopped     bool
}

func (m *MockTrayManager) Start(config Config) error {
	m.StartCalls++
	m.Started = true
	m.StartConfig = config
	return m.StartErr
}

func (m *MockTrayManager) Stop() error {
	m.StopCalls++
	m.Stopped = true
	return m.StopErr
}
