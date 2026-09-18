# Chat adapter validation record

Date: 2026-09-18. Scope: the OpenAI-compatible chat-completions adapter in `internal/llm/infrastructure/chatcompat`.

## Official formats checked

- [OpenAI Chat Completions reference](https://platform.openai.com/docs/api-reference/chat/object?lang=ruby): `POST /v1/chat/completions`, `model`, `messages`, optional `stream`, `response_format`, and `choices[].message.content` / `choices[].delta.content` response fields.
- [DeepSeek Chat Completions reference](https://api-docs.deepseek.com/api/create-chat-completion/): the same request path and compatible message, response, JSON-output, and SSE `data: [DONE]` conventions.

The adapter intentionally targets this shared subset, not the newer provider-specific Responses APIs or tool-call schemas. `openai.go` constructs the documented path and fields, requires a completed `choices` response, and maps token usage. `stream.go` accepts data-only SSE, processes incremental `delta.content`, requires a `stop` finish reason and `[DONE]`, and bounds the buffered event. DeepSeek uses the same adapter through its documented OpenAI-compatible endpoint.

## Reproduction

Run the offline contract and resilience tests; they use synthetic HTTP responses and do not contact a provider or require credentials:

```sh
go test ./internal/llm/infrastructure/chatcompat
```

This record verifies wire compatibility and failure handling as of the date above. It does not certify a provider account, model availability, pricing, retention, or semantic answer quality. Recheck the cited official references and repeat the tests when changing the request or response contract.
