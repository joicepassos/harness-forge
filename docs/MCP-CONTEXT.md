# Read-only MCP context contract (T8.3)

HarnessForge can expose its existing context resolver as one read-only MCP
resource. The stdio server implements JSON-RPC initialize, ping, resources/list,
and resources/read for protocol version `2025-06-18`. A client can launch it with:

```sh
harnessforge mcp serve --repository . --budget 4000
```

The resource URI is `forge://context/current`; its `application/json` body is the
resolver's selection plan with source IDs, evidence paths, inclusion/exclusion
reasons, estimated input size, and overflow information. For MCP stdio, each
JSON-RPC message occupies one line. Embedders can use `mcp.Server.Serve` with
their own reader/writer without adopting a client library.

The service reads the checkout and approved Forge knowledge. It does not apply
sync output, change review state, execute project commands, expose tools, or
push content to clients. The positive token budget controls the resolver's
selection estimate; it is not an exact provider tokenizer. Requests are limited
to 1 MiB and resource bodies to slightly less to leave room for JSON-RPC framing.
An invalid repository or resolver failure returns a generic JSON-RPC internal
error without disclosing local filesystem details.

This is a local stdio transport only: there is no remote listener, network
authentication, or client-specific adapter/certification. MCP clients decide
whether and when to read the resource and whether to include its content in a
model request. Listing a resource does not guarantee automatic context loading.
The server advertises only read-only resource capabilities; it does not expose
tools, prompts, subscriptions, or resource-change notifications.
