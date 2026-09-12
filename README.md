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

## Fase 6 — Multi-provider / BYOK

```powershell
go run ./cmd/harnessforge config set provider deepseek
go run ./cmd/harnessforge config get provider
go run ./cmd/harnessforge ask "Olá"
```

A preferência vale para todos os repositórios do usuário. Ela é salva em `harnessforge/preferences.json` dentro do diretório retornado por `os.UserConfigDir` (no Windows, `%APPDATA%`). Apenas o nome do provedor é persistido; as chaves continuam exclusivamente nas variáveis de ambiente. Sem preferência salva, usa OpenAI. A flag `ask --provider` prevalece sobre a preferência sem alterá-la. O modelo continua sendo selecionado por `--model` ou pelo padrão do provedor.

A configuração segue a mesma separação: domínio contém preferências, aplicação valida o provedor por um catálogo e infraestrutura persiste o JSON. Configurar um provedor não exige chave nem faz chamadas à API.

O roteiro detalhado em Go define fase 6 como Multi-provider/BYOK e fase 7 como Harness IR. A fase 5 do roteiro pede saída estruturada **do modelo**; o JSON atual de `analyze` é a saída determinística do scanner e não conclui esse requisito.

## Fase 7 — Harness IR manual

```powershell
go run ./cmd/harnessforge validate
go run ./cmd/harnessforge validate examples/mili.harness.yaml
```

`validate` lê `.harness/harness.yaml` por padrão, não altera arquivos e não chama IA nem executa quality gates. Erros encerram a CLI com código diferente de zero. `init` agora recusa sobrescrever arquivos existentes.

O contrato v1 está em `schemas/harness-v1.schema.json`. Apenas `version: 1` e `project.name` são obrigatórios no documento mínimo. Se presentes, regras exigem `id`, `description`, `origin` (`human` ou `ai`) e `status` (`candidate`, `approved` ou `rejected`). Regras de IA exigem evidências com `file`; `symbol` e `revision` são opcionais. Regras humanas não exigem pontuação de confiança. IDs devem ser únicos dentro de cada coleção. O schema descreve a estrutura; a validação do domínio também verifica unicidade.

Seções opcionais: `project.languages`, `architecture.styles`, `rules`, `skills` (id/description) e `quality_gates` (id/command). Escopos usam `scope.paths`; nesta fase são apenas metadados. Evidências são referências declaradas: validar não prova sua veracidade nem a existência dos arquivos. `approved` é uma declaração manual, não uma aprovação autenticada.

O exemplo Mili contém uma regra candidata ilustrativa. Ele não altera as decisões do projeto Mili. O validador preserva os bytes originais; edição automática e escrita com preservação de comentários ficam para uma etapa futura.
