package consul

// Dummy is struct for implements TTLUpdater interface
type Dummy struct{}

// NewDummyClient return initialized struct Dummy
func NewDummyClient() *Dummy {
	return &Dummy{}
}

// UpdateTTL implements TTLUpdater interface.
// Return always nil.
func (c *Dummy) UpdateTTL(checkID, status, output string) error {
	return nil
}
