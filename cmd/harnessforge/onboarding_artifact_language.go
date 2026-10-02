package main

import (
	"fmt"
	"strings"
)

var setupInstructionPortuguese = strings.NewReplacer(
	"agent instructions", "instrucoes para agentes",
	"## Project context", "## Contexto do projeto",
	"Team observations:", "Observacoes da equipe:",
	"Additional documents considered during setup:", "Documentos adicionais considerados na configuracao:",
	"## Architecture", "## Arquitetura",
	"## Scope and precedence", "## Escopo e precedencia",
	"- Rules without path scopes apply globally.", "- Regras sem escopo de caminho se aplicam globalmente.",
	"- Directory subtree scopes are emitted as nested `AGENTS.md` files and are loaded when Codex runs with its current directory in that subtree. Codex combines ancestor instructions; HarnessForge does not resolve conflicts between them.", "- Escopos de subarvores de diretorio geram arquivos `AGENTS.md` aninhados. O Codex combina instrucoes ancestrais; o HarnessForge nao resolve conflitos entre elas.",
	"- Other path globs retain their exact authored text and are advisory; Codex does not enforce file-glob matching from this export.", "- Outros globs de caminho sao apenas orientacoes; o Codex nao aplica correspondencia de globs de arquivo nesta exportacao.",
	"- Global and directory-scoped rules coexist; conflicting rules require explicit reconciliation.", "- Regras globais e de diretorio coexistem; conflitos exigem conciliacao explicita.",
	"## Approved rules", "## Regras aprovadas",
	"## Global rules", "## Regras globais",
	"## Quality commands", "## Comandos de qualidade",
	" (reference: `", " (referencia: `",
)

func localizeSetupInstructions(content []byte) []byte {
	return []byte(setupInstructionPortuguese.Replace(string(content)))
}

// The language of generated instructions is independent of the interface language.
func (s setupSession) chooseArtifactLanguage() (string, error) {
	if s.interactive {
		return s.formSelect("Language of generated skills and agent instructions / Idioma das skills e instrucoes geradas", []string{"Português (Brasil)", "English"}, []string{"pt-BR", "en"}, "pt-BR")
	}
	for {
		answer, err := s.ask("Language of generated skills and agent instructions / Idioma das skills e instrucoes geradas [1 Português, 2 English] (default 1): ")
		if err != nil {
			return "", err
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "", "1", "pt", "pt-br", "portugues", "português":
			return "pt-BR", nil
		case "2", "en", "english":
			return "en", nil
		default:
			fmt.Fprintln(s.output, "Choose 1 or 2 / Escolha 1 ou 2.")
		}
	}
}
