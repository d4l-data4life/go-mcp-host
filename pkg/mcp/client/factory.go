package client

import (
	"github.com/d4l-data4life/go-mcp-host/pkg/config"
	"github.com/d4l-data4life/go-mcp-host/pkg/mcp/protocol"
	"github.com/d4l-data4life/go-mcp-host/pkg/mcp/transport"
	"github.com/pkg/errors"
)

// Factory creates MCP clients based on configuration
type Factory struct {
	clientName    string
	clientVersion string
}

// NewFactory creates a new client factory
func NewFactory(clientName, clientVersion string) *Factory {
	return &Factory{
		clientName:    clientName,
		clientVersion: clientVersion,
	}
}

// CreateClient creates a new MCP client based on server configuration
func (f *Factory) CreateClient(serverConfig config.MCPServerConfig) (*Client, error) {
	var trans transport.Transport
	var err error

	switch serverConfig.Type {
	case "stdio":
		trans, err = f.createStdioTransport(serverConfig)
	case "http":
		trans, err = f.createHTTPTransport(serverConfig)
	default:
		return nil, errors.Errorf("unsupported transport type: %s", serverConfig.Type)
	}

	if err != nil {
		return nil, errors.Wrapf(err, "failed to create transport for server %s", serverConfig.Name)
	}

	clientConfig := ClientConfig{
		ClientName:    f.clientName,
		ClientVersion: f.clientVersion,
		Capabilities: protocol.ClientCapabilities{
			Roots: &protocol.RootsCapability{
				ListChanged: true,
			},
			Sampling: map[string]interface{}{},
		},
	}

	client := NewClient(trans, clientConfig)
	return client, nil
}

// createStdioTransport creates a stdio transport from configuration
func (f *Factory) createStdioTransport(serverConfig config.MCPServerConfig) (transport.Transport, error) {
	if serverConfig.Command == "" {
		return nil, errors.New("stdio transport requires command")
	}

	// Convert env map to slice
	envSlice := make([]string, 0, len(serverConfig.Env))
	for key, value := range serverConfig.Env {
		envSlice = append(envSlice, key+"="+value)
	}

	return transport.NewStdioTransport(serverConfig.Command, serverConfig.Args, envSlice)
}

// createHTTPTransport creates an HTTP transport from configuration
func (f *Factory) createHTTPTransport(serverConfig config.MCPServerConfig) (transport.Transport, error) {
	if serverConfig.URL == "" {
		return nil, errors.New("http transport requires URL")
	}

	// TODO: Get TLS skip verify from config
	tlsSkipVerify := false

	return transport.NewHTTPTransport(serverConfig.URL, serverConfig.Headers, tlsSkipVerify)
}
