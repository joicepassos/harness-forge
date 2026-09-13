package infrastructure

import "harnessforge/internal/localization/domain"

type Catalog struct{}

func (Catalog) Supports(language domain.Language) bool {
	return language == domain.English || language == domain.BrazilianPortuguese
}

func (Catalog) Text(language domain.Language, key string) string {
	if language == domain.BrazilianPortuguese {
		if translated, ok := portuguese[key]; ok {
			return translated
		}
	}
	return english[key]
}

var english = map[string]string{
	"root.short":               "HarnessForge creates and maintains coding-agent harnesses",
	"version.short":            "Print the HarnessForge version",
	"init.short":               "Create the initial harness configuration",
	"analyze.short":            "Analyze a repository",
	"ask.short":                "Ask an AI provider",
	"config.short":             "Manage user preferences (credentials stay in environment variables)",
	"config.set.short":         "Save the default AI provider",
	"config.get.short":         "Show the default AI provider",
	"validate.short":           "Validate a manually edited Harness IR YAML file",
	"review.short":             "Review a rule while preserving manual YAML formatting",
	"context.short":            "Inspect model context",
	"context.explain.short":    "Explain repository context selection",
	"skill.short":              "Discover reviewed repository procedures",
	"skill.discover.short":     "Propose recurring development procedures",
	"skill.generate.short":     "Generate an approved skill",
	"eval.short":               "Evaluate retrieval and grounded answers",
	"eval.run.short":           "Run deterministic evaluation",
	"eval.compare.short":       "Compare evaluation reports",
	"doctor.short":             "Diagnose harness health without modifying files",
	"github.short":             "Read GitHub discussions without changing them",
	"github.learn.short":       "Extract reviewable knowledge from GitHub",
	"drift.short":              "Compare approved harness rules with a repository without changing either",
	"output.created":           "Created %s\n",
	"output.valid":             "Valid Harness IR: %s\n",
	"output.rule":              "Rule %s: %s\n",
	"output.skill":             "Generated skill %s\n",
	"error.unsupported_format": "unsupported format %q",
}

var portuguese = map[string]string{
	"root.short":               "HarnessForge cria e mantém configurações para agentes de programação",
	"version.short":            "Exibir a versão do HarnessForge",
	"init.short":               "Criar a configuração inicial do harness",
	"analyze.short":            "Analisar um repositório",
	"ask.short":                "Consultar um provedor de IA",
	"config.short":             "Gerenciar preferências do usuário (credenciais ficam em variáveis de ambiente)",
	"config.set.short":         "Salvar o provedor de IA padrão",
	"config.get.short":         "Exibir o provedor de IA padrão",
	"validate.short":           "Validar um arquivo YAML do Harness IR editado manualmente",
	"review.short":             "Revisar uma regra preservando a formatação manual do YAML",
	"context.short":            "Inspecionar o contexto do modelo",
	"context.explain.short":    "Explicar a seleção de contexto do repositório",
	"skill.short":              "Descobrir procedimentos revisados do repositório",
	"skill.discover.short":     "Propor procedimentos recorrentes de desenvolvimento",
	"skill.generate.short":     "Gerar uma habilidade aprovada",
	"eval.short":               "Avaliar recuperação e respostas fundamentadas",
	"eval.run.short":           "Executar avaliação determinística",
	"eval.compare.short":       "Comparar relatórios de avaliação",
	"doctor.short":             "Diagnosticar a saúde do harness sem modificar arquivos",
	"github.short":             "Ler discussões do GitHub sem modificá-las",
	"github.learn.short":       "Extrair conhecimento revisável do GitHub",
	"drift.short":              "Comparar regras aprovadas do harness com um repositório sem modificar nenhum dos dois",
	"output.created":           "Criado %s\n",
	"output.valid":             "Harness IR válido: %s\n",
	"output.rule":              "Regra %s: %s\n",
	"output.skill":             "Habilidade gerada %s\n",
	"error.unsupported_format": "formato não suportado %q",
}
