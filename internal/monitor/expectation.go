package monitor

type expectation struct {
	id            int
	criteria      string
	expectedValue string
	endpointID    int
}
