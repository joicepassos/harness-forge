package main

import (
	"fmt"
	"io"
	"strings"
)

// showSetupReview keeps generated file contents intact while giving the
// interactive review a consistent, scannable frame. It also works without
// colors when output is redirected or accessibility mode is enabled.
func showSetupReview(output io.Writer, plan setupPlan) {
	plainOutput := output
	portuguese := false
	if localized, ok := output.(setupLocalizedWriter); ok {
		plainOutput = localized.output
		portuguese = true
	}
	p := presentationFor(plainOutput)
	label := func(en, pt string) string {
		if portuguese {
			return pt
		}
		return en
	}

	fmt.Fprintln(plainOutput)
	fmt.Fprintln(plainOutput, p.brand()+"  "+p.heading(label("Setup review", "Revisao da configuracao")))
	fmt.Fprintln(plainOutput, p.paint(strings.Repeat("─", 56), "#526171", false))
	fmt.Fprintln(plainOutput, p.status("warning", label("Preview only. No project files have been changed.", "Apenas uma previa. Nenhum arquivo do projeto foi alterado.")))
	if plan.Summary != "" {
		fmt.Fprintf(plainOutput, "\n%s\n%s\n", p.heading(label("Project", "Projeto")), plan.Summary)
	}
	fmt.Fprintf(plainOutput, "\n%s: %s\n", label("Languages", "Linguagens"), strings.Join(plan.Harness.Project.Languages, ", "))
	fmt.Fprintf(plainOutput, "%s: %s\n", label("Architecture", "Arquitetura"), strings.Join(plan.Harness.Architecture.Styles, ", "))
	fmt.Fprintf(plainOutput, "%s: %d  ·  %s: %d  ·  %s: %d\n",
		label("Rules", "Regras"), len(plan.Harness.Rules),
		label("Skills", "Skills"), len(plan.Harness.Skills),
		label("Context documents", "Documentos de contexto"), len(plan.Documents))

	fmt.Fprintf(plainOutput, "\n%s (%d)\n", p.heading(label("Files to review", "Arquivos para revisar")), len(plan.Files))
	for index, file := range plan.Files {
		action := label("Create", "Criar")
		if file.Append {
			action = label("Append to existing instructions", "Acrescentar as instrucoes existentes")
		} else if file.Replace {
			action = label("Replace generated starter file", "Substituir arquivo inicial gerado")
		}
		fmt.Fprintf(plainOutput, "\n%s\n", p.paint(strings.Repeat("─", 56), "#526171", false))
		fmt.Fprintf(plainOutput, "%s  %s\n", p.accent(fmt.Sprintf("%02d/%02d", index+1, len(plan.Files))), p.heading(file.Path))
		fmt.Fprintln(plainOutput, action)
		fmt.Fprintln(plainOutput)
		fmt.Fprintf(plainOutput, "%s\n", file.Content)
	}
	fmt.Fprintln(plainOutput, p.paint(strings.Repeat("─", 56), "#526171", false))
	fmt.Fprintln(plainOutput, p.status("warning", label("Review the files above before choosing an action.", "Revise os arquivos acima antes de escolher uma acao.")))
}
