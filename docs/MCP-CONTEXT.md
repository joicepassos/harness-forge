# Read-only MCP context contract (T8.3)

HarnessForge's optional context resolver is a read-only operation over a
repository checkout and its approved Forge knowledge. It returns a serialized
selection plan with source IDs, evidence paths, inclusion/exclusion reasons,
estimated input size, and explicit overflow. It never applies sync output,
changes review state, runs project commands, or pushes context to a client.

The current transport-neutral API is `internal/mcp.Resolver`. A future MCP
transport may expose it as a tool and a resource under protocol version
`2025-06-18`. Clients choose whether and when to call/read it; resource support
does not guarantee that a client will load the content or preserve its budget.
The caller must provide a positive budget. The local byte upper bound is an
estimate, not a provider tokenizer. Provider-specific counters require an
explicit implementation and separate data handling decision.

Limits: this package does not implement JSON-RPC, sessions, server discovery,
authentication, remote serving, or client compatibility. Keep the service local
and read-only until transport security and real client support are implemented.
