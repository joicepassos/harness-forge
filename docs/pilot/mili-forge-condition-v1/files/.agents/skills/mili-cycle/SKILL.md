---
name: mili-cycle
description: Executar um ciclo funcional do Mili envolvendo frontend, backend e persistência, com contratos compartilhados e validação do fluxo real.
---

# Ciclo integrado Mili

Escolha um resultado observável para o usuário com base no código atual. Consulte o [contexto de produto](../../../.codex/context/product.md) somente quando a decisão envolver escopo ou domínio.

1. Inspecione mudanças existentes, contratos HTTP, migrations e comandos de execução disponíveis; preserve trabalho paralelo.
2. Defina entradas, resultados, estados vazios e erros do fluxo. Combine contrato HTTP antes de dividir frontend e backend.
3. Delegue partes independentes com arquivos sob responsabilidade de cada agente; envie caminhos e decisões relevantes, sem copiar todo o histórico.
4. Implemente persistência e interface no mesmo ciclo. Use dados reais do backend e mensagens compreensíveis; não substitua integração por resultados simulados silenciosamente.
5. Execute migrations e testes pertinentes ao comportamento alterado. Para mudanças relacionais, verifique PostgreSQL real; para frontend, execute verificação de tipos/build e percorra o fluxo quando houver navegador disponível.
6. Revise integração, validações, erros e integridade dos dados. Se execução estiver bloqueada, relate a limitação e os testes efetivamente realizados sem declarar conclusão integral.

Confirme comandos nos arquivos atuais em vez de memorizar versões ou caminhos de runtimes. Não habilite serviços de produção para contornar um problema local.

Reporte implementação, principal trade-off, testes realizados e pendências relevantes. Atualize instruções recorrentes apenas quando isso evitar retrabalho demonstrável.
