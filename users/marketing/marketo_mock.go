package marketing

// MockGoketoClient is only used for testing.
type MockGoketoClient struct {
	LatestReq []byte
}

// RefreshToken does nothing.
func (m MockGoketoClient) RefreshToken() error {
	return nil
}

// Post returns a fake successful response.
func (m *MockGoketoClient) Post(resource string, data []byte) ([]byte, error) {
	m.LatestReq = data
	return []byte("{\"requestId\": \"foo\",\"result\": [{\"id\": 1337,\"status\": \"created\"}],\"success\": true}"), nil
}
