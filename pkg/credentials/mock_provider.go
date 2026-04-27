package credentials

import "context"

// MockProvider is a test helper that returns canned credentials.
// Thread-safe for concurrent test usage.
type MockProvider struct {
	ProviderName string            // defaults to "mock" if empty
	Credentials  map[string]string // name -> value
	StoreErr     error             // error to return from Store
	AvailableVal bool              // return value for Available (default: true)

	// Recorded calls for assertions
	ResolveCalls []string
	StoreCalls   []MockStoreCall
}

// MockStoreCall records a Store invocation.
type MockStoreCall struct {
	Name  string
	Value string
}

// NewMockProvider creates a mock provider with the given canned credentials.
func NewMockProvider(credentials map[string]string) *MockProvider {
	return &MockProvider{
		Credentials:  credentials,
		AvailableVal: true,
	}
}

// Name returns the provider name (default "mock").
func (m *MockProvider) Name() string {
	if m.ProviderName != "" {
		return m.ProviderName
	}
	return "mock"
}

// Resolve returns the canned credential or ErrNotFound.
func (m *MockProvider) Resolve(_ context.Context, name string) (*Credential, error) {
	m.ResolveCalls = append(m.ResolveCalls, name)

	value, ok := m.Credentials[name]
	if !ok {
		return nil, ErrNotFound
	}
	return &Credential{
		Name:   name,
		Value:  value,
		Source: m.Name(),
	}, nil
}

// Store records the call and returns StoreErr.
func (m *MockProvider) Store(_ context.Context, name, value string) error {
	m.StoreCalls = append(m.StoreCalls, MockStoreCall{Name: name, Value: value})
	if m.StoreErr != nil {
		return m.StoreErr
	}
	// Actually store it so subsequent Resolve calls find it
	if m.Credentials == nil {
		m.Credentials = make(map[string]string)
	}
	m.Credentials[name] = value
	return nil
}

// Available returns the configured value (default true).
func (m *MockProvider) Available() bool {
	return m.AvailableVal
}
