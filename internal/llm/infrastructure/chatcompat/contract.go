package chatcompat

import "harnessforge/internal/llm/domain"

// The transport maps its wire format to the domain contract.
type Provider = domain.Provider
type Request = domain.Request
type Response = domain.Response
