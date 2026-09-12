# HarnessForge

CLI Go para analisar repositórios, consultar IAs e manter instruções de agentes em um formato neutro e editável.

## Instalação e primeiros comandos

Baixe o binário da release correspondente ao seu sistema ou execute a partir do código com Go 1.23+:

```powershell
go run ./cmd/harnessforge version
go run ./cmd/harnessforge init
go run ./cmd/harnessforge analyze --git --format json C:\caminho\para\seu-projeto
```

`init` cria `.harness/harness.yaml` e recusa sobrescrever arquivos existentes. `analyze` faz análise local determinística. Com `--git`, inclui branches locais/remotos conhecidos, arquivos pendentes, autores, contagem de commits e mensagens/arquivos dos últimos 20 commits. Repositórios sem commits são aceitos; erros Git são reportados, não descartados. Nenhum fetch é executado.

## IA e BYOK

```powershell
go run ./cmd/harnessforge config set provider deepseek
go run ./cmd/harnessforge config get provider
go run ./cmd/harnessforge ask "Olá"
go run ./cmd/harnessforge ask --stream "Explique Strategy em uma frase"
go run ./cmd/harnessforge ask --format json --repository C:\caminho\para\seu-projeto "Liste padrões sustentados pelo relatório"
```

| Provedor | Variável de ambiente | Modelo padrão |
| --- | --- | --- |
| openai | OPENAI_API_KEY | gpt-4o-mini |
| deepseek | DEEPSEEK_API_KEY | deepseek-v4-flash |
| gemini | GEMINI_API_KEY | Informar --model |
| groq | GROQ_API_KEY | Informar --model |
| ollama | Nenhuma no servidor local | Informar --model |

As chaves ficam exclusivamente no ambiente do processo; `.env` não é carregado. O Ollama deve estar em execução em `localhost:11434` com o modelo instalado. A preferência global é salva em `harnessforge/preferences.json` sob `os.UserConfigDir` (Windows: `%APPDATA%`). Só o nome do provedor é persistido. `--provider` prevalece para uma execução; sem preferência, usa OpenAI.

O modo texto envia somente o prompt. `--repository`, disponível com `--format json`, envia o relatório determinístico do scanner ao provedor escolhido, sem enviar o conteúdo completo dos arquivos. Nenhuma consulta modifica o repositório.

### Saída estruturada da IA

`ask --format json` solicita JSON ao provedor e valida localmente `schemas/architecture-analysis.schema.json`. A saída contém `architecture` e `patterns`; cada padrão exige nome, confiança entre 0 e 1 e citações literais do prompt ou relatório enviado. Campos extras, chaves duplicadas, saída truncada, citações inexistentes e JSON inválido são rejeitados antes de escrever em stdout.

Os estilos arquiteturais v1 são `ddd`, `hexagonal`, `layered`, `clean-architecture`, `event-driven`, `microservices`, `monolith` e `mvc`. Cada estilo precisa de um padrão de mesmo nome com evidência. Tecnologias pertencem a `patterns`. Na ausência de evidência o modelo pode retornar arrays vazios.

Confiança é uma estimativa não calibrada do modelo, não frequência observada. A presença literal de uma citação não prova que a interpretação arquitetural é correta; revisão humana continua necessária.

O adaptador usa JSON mode e validação local, sem depender de suporte nativo a JSON Schema em todos os provedores. Compatibilidade depende do modelo escolhido. `--stream` vale apenas para texto: JSON é retido até ser validado.

### Falhas e cancelamento

Há até 3 tentativas para HTTP 429, 500, 502, 503 e 504, com espera progressiva e respeito a `Retry-After`. Se a espera solicitada exceder 10 segundos, a falha é devolvida ao usuário. Timeout total por chamada: 60 segundos. Erros de autenticação/saldo, conexão ambígua, conteúdo inválido e streams já iniciados não são repetidos. Ctrl+C cancela a operação. Streaming exige o terminador e uma conclusão normal; falhas após trechos exibidos são reportadas.

## Harness IR — edição manual e revisão

```powershell
go run ./cmd/harnessforge validate
go run ./cmd/harnessforge validate examples/sample.harness.yaml
go run ./cmd/harnessforge validate caminho/harness.yaml --repository C:\caminho\para\seu-projeto
go run ./cmd/harnessforge review minha-regra approved --file caminho/harness.yaml
go run ./cmd/harnessforge review minha-regra candidate --file caminho/harness.yaml
```

`validate` lê `.harness/harness.yaml` por padrão. Valida o schema embutido `schemas/harness-v1.schema.json` e as invariantes do domínio. Apenas `version: 1` e `project.name` são obrigatórios no documento mínimo. Seções opcionais: `project.languages`, `architecture.styles`, `rules`, `skills` (id/description), `quality_gates` (id/command).

Regras exigem `id`, `description`, `origin` (`human`/`ai`) e `status` (`candidate`/`approved`/`rejected`). IDs são únicos por coleção. Regras de IA exigem evidências com `file`; `symbol` e `revision` são opcionais. Regras humanas não exigem confiança nem evidência de padrões existentes. `scope.paths` é metadado; não executa filtros nesta fase.

`validate --repository` verifica arquivos e presença literal de símbolos. Com `revision`, verifica o conteúdo naquela revisão Git. Caminhos devem ser relativos à raiz do repositório; escapes de diretório são rejeitados. Isso não substitui análise AST nem prova a veracidade da regra. Quality gates são apenas declarados, nunca executados por validate/review.

`review` permite candidate → approved/rejected e approved/rejected → candidate. Uma decisão deve ser reaberta antes de ser invertida. A edição modifica apenas o escalar de status, preservando comentários, ordem, aspas e quebras de linha; usa arquivo temporário, bloqueio entre operações de review e detecção de alterações concorrentes. Status com aliases, anchors ou sintaxe multilinha devem ser editados manualmente. Não existe reserialização geral do YAML nesta versão.

A edição manual pode declarar qualquer status válido; não há autenticação de aprovador. O histórico Git fornece rastreabilidade. O exemplo incluído é ilustrativo e contém uma regra candidata, não uma decisão adotada no projeto.

## Arquitetura e validação

Domínio define contratos, mensagens, Harness e transições; aplicação coordena casos de uso; infraestrutura implementa HTTP, YAML e persistência. Provider é a Strategy; provedores de mesmo protocolo compartilham o adaptador Chat Completions. A CLI compõe as dependências.

```powershell
go test ./...
go vet ./...
```

Testes cobrem schemas, citações, falhas HTTP, retry, streaming truncado, Git com/sem commits, revisão e preservação de edição manual. DeepSeek foi validado ao vivo em JSON e streaming. OpenAI, Gemini, Groq e Ollama têm testes simulados; execução real exige saldo/credenciais/servidor de cada ambiente. Anthropic permanece uma integração opcional não implementada.

Referências de protocolo: [OpenAI](https://developers.openai.com/api/docs/guides/structured-outputs), [DeepSeek](https://api-docs.deepseek.com/api/create-chat-completion/), [Gemini](https://ai.google.dev/gemini-api/docs/openai), [Groq](https://console.groq.com/docs/openai), [Ollama](https://docs.ollama.com/api/openai-compatibility).
