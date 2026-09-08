package mcp

import "encoding/json"

const (
	JSONRPCVersion = "2.0"
	MCPVersion     = "2024-11-05"

	// Standard JSON-RPC 2.0 Error Codes
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// Request is a JSON-RPC 2.0 request or notification message.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a successful JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result"`
}

// ErrorResponse is an error JSON-RPC 2.0 response.
type ErrorResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Error   *RPCError `json:"error"`
}

// RPCError captures JSON-RPC 2.0 error details.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// InitializeParams contains parameters for the initialize method.
type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
}

// ClientCapabilities represents capabilities provided by the MCP client.
type ClientCapabilities struct {
	Roots        *RootsCapability `json:"roots,omitempty"`
	Sampling     map[string]any   `json:"sampling,omitempty"`
	Experimental map[string]any   `json:"experimental,omitempty"`
}

// RootsCapability represents client root list capabilities.
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ClientInfo contains client name and version.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// InitializeResult is returned in response to the initialize method.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// ServerCapabilities lists the features supported by this server.
type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// ToolsCapability indicates support for MCP tools.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerInfo identifies the server name and version.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tool describes an MCP tool available to agents.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema ToolInputSchema `json:"inputSchema"`
}

// ToolInputSchema defines JSON schema for tool arguments.
type ToolInputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]PropertyDef `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

// PropertyDef describes a single schema property.
type PropertyDef struct {
	Type        string         `json:"type,omitempty"`
	Description string         `json:"description,omitempty"`
	Enum        []string       `json:"enum,omitempty"`
	Items       map[string]any `json:"items,omitempty"`
}

// ListToolsResult lists all available tools.
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams defines arguments passed to tools/call.
type CallToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// CallToolResult contains content returned by tools/call.
type CallToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ToolContent encapsulates a piece of returned tool content.
type ToolContent struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}
