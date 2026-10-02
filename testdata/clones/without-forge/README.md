# Clone de consumo sem Forge

Esta fixture representa dois clones do mesmo projeto consumidor. Cada um contém
somente o export estático destinado ao agente correspondente e os arquivos do
projeto. Não há configuração `.harness`/`.forge`, manifesto de geração nem
dependência da CLI HarnessForge.

- `codex/AGENTS.md` é descoberto pelo Codex como instrução do repositório.
- `claude/CLAUDE.md` é o perfil nativo do Claude Code.

Os exports são versionados junto com o código do consumidor. A fonte Forge
revisável fica em `../payments-demo-forge`; o teste de aceitação compila essa
fonte, sincroniza uma cópia temporária e compara todos os arquivos nativos com
estes clones. Para demonstrar o uso, peça ao agente no diretório `codex` ou
`claude` para implementar a nova opção de pagamento descrita em `README.md`.
As instruções apontam para a skill nativa versionada em cada clone; assim, o
agente obtém as convenções do próprio clone, sem buscar ou executar o Forge.

`docs/payment-provider-change.md` é uma cópia documental mantida para leitura
humana. Os agentes consomem as skills nativas em `.agents/skills/` (Codex) e
`.claude/skills/` (Claude Code), que são as saídas verificadas pelo teste.
