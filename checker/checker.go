package checker

// Checkup is adapter for function the Check
type Checkup func() error

// Verify is function for sequential execution of Checkup
// and stops on first error or on end of slice
func Verify(cf []Checkup) error {
	for _, f := range cf {
		if err := f(); err != nil {
			return err
		}
	}

	return nil
}
