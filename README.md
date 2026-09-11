# HarnessForge

CLI para analisar repositórios e consultar modelos de IA.

```powershell
go run ./cmd/harnessforge analyze --git --format json C:\Users\joice\mili
go run ./cmd/harnessforge ask --provider deepseek "Responda em português: olá"
go run ./cmd/harnessforge ask --provider gemini --model "MODELO_DA_SUA_CONTA" "Olá"
go run ./cmd/harnessforge ask --provider groq --model "MODELO_DA_SUA_CONTA" "Olá"
go run ./cmd/harnessforge ask --provider ollama --model "MODELO_INSTALADO" "Olá"
```

| Provedor | Variável de ambiente | Modelo padrão |
| --- | --- | --- |
| openai | OPENAI_API_KEY | gpt-4o-mini |
| deepseek | DEEPSEEK_API_KEY | deepseek-v4-flash |
| gemini | GEMINI_API_KEY | Informar --model |
| groq | GROQ_API_KEY | Informar --model |
| ollama | Nenhuma para servidor local | Informar --model |

O provedor padrão é OpenAI. O Ollama precisa estar em execução em localhost:11434, com o modelo já instalado. As chaves são lidas do ambiente do processo; arquivos .env não são carregados. `ask` recebe o texto fornecido como prompt e não lê o repositório automaticamente.

## Arquitetura

A integração de IA usa Strategy e separação de responsabilidades inspirada em DDD:

- `internal/llm/domain`: contrato Provider e mensagens do domínio, sem dependência de HTTP ou ambiente.
- `internal/llm/application`: caso de uso Ask, que depende das interfaces Provider e ProviderResolver.
- `internal/llm/infrastructure/chatcompat`: registro de provedores e adaptador HTTP compartilhado para APIs compatíveis com Chat Completions.
- `internal/llm`: composição das dependências usadas pela CLI.

Cada estratégia implementa Provider.Generate. Provedores que compartilham o protocolo reutilizam o adaptador com configurações diferentes. Para um protocolo diferente, implemente Provider em outro adaptador e componha um ProviderResolver que o selecione, preservando o caso de uso e o domínio.

Compatibilidade de protocolo: [Gemini](https://ai.google.dev/gemini-api/docs/openai), [Groq](https://console.groq.com/docs/openai), [Ollama](https://docs.ollama.com/api/openai-compatibility).

## Validação

```powershell
go test ./...
go vet ./...
```

Os testes de contrato HTTP usam respostas simuladas. A integração DeepSeek também foi validada com uma chamada real usando o relatório do Mili. Gemini, Groq e Ollama exigem validação real no ambiente configurado pelo usuário.
