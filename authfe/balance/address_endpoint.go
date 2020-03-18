package balance

// addressEndpoint is a very simple endpoint.
type addressEndpoint struct {
	address string
}

// Assert it implements Endpoint
var _ Endpoint = &addressEndpoint{}

// Key implements Endpoint.
func (e *addressEndpoint) Key() string {
	return e.address
}

// HostAndPort implements Endpoint.
func (e *addressEndpoint) HostAndPort() string {
	return e.address
}

// String implements fmt.Stringer.
func (e *addressEndpoint) String() string {
	return e.address
}
