package infrastructure

import (
	"harnessforge/internal/localization/domain"
	"strings"
)

type Catalog struct{}

func (Catalog) Supports(language domain.Language) bool {
	return language == domain.English || language == domain.BrazilianPortuguese || language == domain.Spanish
}

func (Catalog) Text(language domain.Language, key string) string {
	catalogs := map[domain.Language]map[string]string{domain.BrazilianPortuguese: portuguese, domain.Spanish: spanish}
	if translated, ok := catalogs[language][key]; ok {
		return translated
	}
	if value, ok := english[key]; ok {
		return value
	}
	return key
}

func (Catalog) Presentation(language domain.Language, value string) string {
	if language == domain.Spanish {
		if translated, ok := presentationES[value]; ok {
			return translated
		}
		return strings.NewReplacer("Available Commands:", "Comandos disponibles:", "Global Flags:", "Opciones globales:", "Flags:", "Opciones:", "Usage:", "Uso:", "help for ", "ayuda para ").Replace(value)
	}
	if language != domain.BrazilianPortuguese {
		return value
	}
	if translated, ok := presentationPT[value]; ok {
		return translated
	}
	return strings.NewReplacer("Available Commands:", "Comandos disponíveis:", "Global Flags:", "Opções globais:", "Flags:", "Opções:", "Usage:", "Uso:", "help for ", "ajuda para ").Replace(value)
}

var presentationPT = map[string]string{
	"Help about any command":                                                       "Ajuda sobre qualquer comando",
	"Generate the autocompletion script for the specified shell":                   "Gerar o script de preenchimento automático para o shell especificado",
	"Language for CLI help and common output (en, pt-BR or es)":                    "Idioma da ajuda e das mensagens comuns (en, pt-BR ou es)",
	"Include Git repository metadata":                                              "Incluir metadados Git do repositório",
	"Output format: text or json":                                                  "Formato de saída: text ou json",
	"Verify evidence files and literal symbols in this repository":                 "Verificar arquivos de evidência e símbolos literais neste repositório",
	"Repository to inspect without modifying it":                                   "Repositório a inspecionar sem modificar",
	"Verify evidence against this repository without modifying it":                 "Verificar evidências neste repositório sem modificar",
	"Print reviewable correction proposals without applying them":                  "Exibir propostas de correção para revisão sem aplicar",
	"Harness YAML file":                                                            "Arquivo YAML do harness",
	"Model override":                                                               "Modelo alternativo",
	"Override saved provider":                                                      "Substituir o provedor salvo",
	"Output: text or validated json":                                               "Saída: text ou json validado",
	"Include selected repository context as evidence (JSON only)":                  "Incluir contexto do repositório como evidência (somente JSON)",
	"Maximum estimated tokens for repository context; 0 selects the model default": "Máximo estimado de tokens para o contexto; 0 seleciona o padrão do modelo",
	"Print repository context selection and do not call the provider":              "Exibir seleção de contexto sem consultar o provedor",
	"Stream text as it arrives":                                                    "Transmitir texto à medida que chega",
	"Model name used to choose the default budget":                                 "Nome do modelo para selecionar o orçamento padrão",
	"Confirm human review of this proposal":                                        "Confirmar revisão humana desta proposta",
	"Environment variable containing a GitHub token":                               "Variável de ambiente contendo um token do GitHub",
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
