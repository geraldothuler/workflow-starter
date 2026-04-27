package exporter

import (
	"time"

	"github.com/Cobliteam/workflow-toolkit/pkg/transport"
)

// HTTPTransport is an alias for the shared transport.HTTPTransport.
// Kept for backward compatibility within the exporter package.
type HTTPTransport = transport.HTTPTransport

// NewHTTPTransport creates an HTTP transport. Delegates to transport.NewHTTPTransport.
func NewHTTPTransport(baseURL, authType, authValue string, headers map[string]string, timeout time.Duration) *HTTPTransport {
	return transport.NewHTTPTransport(baseURL, authType, authValue, headers, timeout)
}
