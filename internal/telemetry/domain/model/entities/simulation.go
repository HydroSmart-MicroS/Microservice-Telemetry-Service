package entities

type SimulationState struct {
	Running   bool
	Seeding   bool
	Completed int
	Total     int
	Error     string
}
