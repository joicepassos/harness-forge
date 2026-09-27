# Clone de consumo sem Forge

Esta fixture representa dois clones do mesmo projeto consumidor. Cada um contém
somente o export estático destinado ao agente correspondente e os arquivos do
projeto. Não há configuração `.harness`/`.forge`, manifesto de geração nem
dependência da CLI HarnessForge.

- `codex/AGENTS.md` é descoberto pelo Codex como instrução do repositório.
- `claude/CLAUDE.md` é o perfil nativo do Claude Code.

Os exports são versionados junto com o código do consumidor. Para demonstrar o
uso, peça ao agente no diretório `codex` ou `claude` para implementar a nova
opção de pagamento descrita em `README.md`. As instruções requerem preservar a
regra de idempotência e validar os workspaces Go indicados. Assim, o agente
obtém as convenções do próprio clone, sem buscar ou executar o Forge.

Os dois arquivos de instrução seguem o formato produzido atualmente pelo
renderizador (`AGENTS.md` e `CLAUDE.md`); seu conteúdo é igual salvo pelo nome do
agente no cabeçalho.
