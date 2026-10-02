# Contexto de produto do Mili

## Confirmado pelo usuário

Mili atende empresas de diferentes setores e portes. O estágio atual é um protótipo funcional, com execução integrada e banco PostgreSQL preparado para evolução. Evitar tanto especialização em SaaS quanto abstrações genéricas sem caso concreto. Autenticação e infraestrutura de produção ficam fora da prioridade atual.

## Domínio adotado no repositório

- Tenant é a empresa cliente do Mili; não confundir com os clientes dessa empresa.
- Operação é uma unidade de negócio que a empresa acompanha e futuramente mede/cobra.
- Webhook pertence à empresa e pode alimentar várias operações da mesma empresa (N:N).
- Métrica cobrável define como medir um tipo de evento; medição transforma eventos processados em uso.
- Billing, fiscal e relatórios são extensões propostas que consomem uso. A existência de documentação não significa implementação.

Consulte código e migrations para saber o que funciona. O ciclo inicial de setembro de 2026 trata da configuração de operações integrada ao painel e ao backend; não declara medição, planos ou faturamento concluídos.

## Primeiro ciclo implementado

- Painel permite criar/selecionar empresa, cadastrar fonte, criar/editar operação, vincular fontes e ativar/pausar. Busca e filtros operam sobre os dados carregados da empresa.
- Ativa significa configuração habilitada. Exige ao menos uma fonte ativa na ativação e edição; pausar uma fonte posteriormente não altera automaticamente o estado da operação. Isso deverá ser definido na implementação de medição.
- V8 mantém nomes de operações únicos por empresa e vínculos N:N dentro da mesma empresa por FKs compostas. UUIDv7 é gerado na aplicação.
- Execução local e testes reproduzíveis: [README](../../README.md). Perfil `prototype` usa PostgreSQL real e dispensa broker para configuração; ingestão e suíte Testcontainers seguem pendentes.
- Validação do ciclo: 79 testes (incluindo PostgreSQL), 27 verificações HTTP, build frontend e percurso no navegador com recarga, busca, filtros e layout móvel. A empresa “Horizonte · demonstração” contém o exemplo criado pelo painel; empresas `QA-` são fixtures identificadas dos testes HTTP.

## Fontes e divergências

Anexos fornecidos: `full_erd_with_operations (1).html` e `swot_plataforma_saas (1).docx`, originalmente em Downloads. O DOCX apresenta hipóteses de abril de 2026; ambos são referências de produto.

- SWOT restringe o público a SaaS, enquanto a orientação atual inclui diferentes setores.
- ERD propõe Operação → Plano → Produto → MetricRule; os contextos atuais seguem Operação → Webhook compartilhado → Métrica. Não importar o ERD inteiro sem validar um fluxo.
- Keycloak, TimescaleDB, SDKs, gateways, precificação por operações ativas e diferenciais competitivos no SWOT são propostas, não requisitos atuais ou fatos validados.
- Nome indefinido no SWOT está superado: o produto é Mili.
- Saldo de créditos sem ledger e agregado sem dimensão de métrica não bastam para cobrança consistente. Resolver antes de implementar esses módulos.

## Referências seletivas

- Fronteiras entre módulos: [CONTEXT-MAP](../../backend/docs/CONTEXT-MAP.md).
- Operações e webhooks: [Operation](../../backend/docs/operation/CONTEXT.md) e [Webhook](../../backend/docs/webhook/CONTEXT.md).
- Somente ao implementar medição: [Metering](../../backend/docs/metering/CONTEXT.md) e [ADR 0004](../../backend/docs/adr/0004-medicao-exactly-once.md). A deduplicação é por métrica + evento, pois o mesmo evento pode alimentar várias operações.
- Somente ao introduzir identificadores: [ADR 0002](../../backend/docs/adr/0002-identificadores-uuidv7-e-ulid.md).

Atualize este resumo quando uma decisão mudar; registre apenas a diferença relevante, sem copiar documentos inteiros.
