package memory

// DB is an in-memory database for testing, and local development
type DB struct {
}

// New creates a new in-memory database
func New(_, _ string) (*DB, error) {
	return &DB{}, nil
}

// Close finishes using the db. Noop.
func (d *DB) Close() error {
	return nil
}
