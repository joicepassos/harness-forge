package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func chooseSetupInterface(reader *bufio.Reader, output io.Writer, ui ...setupSession) (io.Writer, error) {
	session := setupSession{reader: reader, output: output}
	if len(ui) > 0 && ui[0].interactive {
		language, err := ui[0].formSelect("Interface language / Idioma da interface", []string{"English", "Portugues"}, []string{"en", "pt"}, "en")
		if err != nil {
			return output, err
		}
		if language == "pt" {
			return setupLocalizedWriter{output: output}, nil
		}
		return output, nil
	}
	for {
		answer, err := session.ask("Interface language / Idioma da interface [1 English, 2 Portugues] (default 1): ")
		if err != nil {
			return output, err
		}
		switch strings.ToLower(answer) {
		case "", "1", "en", "english":
			return output, nil
		case "2", "pt", "pt-br", "portugues", "português":
			return setupLocalizedWriter{output: output}, nil
		default:
			fmt.Fprintln(output, "Choose 1 or 2 / Escolha 1 ou 2.")
		}
	}
}

// Translate presentation only; generated files and provider payloads retain their content.
type setupLocalizedWriter struct{ output io.Writer }

func (w setupLocalizedWriter) Write(data []byte) (int, error) {
	_, err := io.WriteString(w.output, setupPortuguese.Replace(string(data)))
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

var setupPortuguese = strings.NewReplacer(
	"File list unavailable for this older run.", "Lista de arquivos indisponivel nesta execucao antiga.",
	"Review complete: choose the next action", "Revisao concluida: escolha a proxima acao",
	"Apply these files", "Aplicar estes arquivos",
	"Back without applying", "Voltar sem aplicar",
	"The background AI proposal is unavailable. Your observations were not stored; a local proposal uses the current project and selected files.", "A proposta de IA em segundo plano nao esta disponivel. Suas observacoes nao foram guardadas; a proposta local usa o projeto atual e os arquivos selecionados.",
	"Call inspector", "Inspetor de chamadas",
	"Real events", "Eventos reais",
	"Esc: cancel request", "Esc: cancelar chamada",
	"The proposal and cited excerpts will be saved in your user cache for later review; credentials and full documents will not be saved there.", "A proposta e os trechos citados serao salvos no cache do seu usuario para revisao posterior; credenciais e documentos completos nao serao salvos ali.",
	"Your observations guide this request but are not retained as raw notes when resuming the proposal.", "Suas observacoes orientam esta chamada, mas nao sao guardadas como notas originais ao retomar a proposta.",
	"Background run started:", "Execucao em segundo plano iniciada:",
	"Review later:", "Revisar depois:",
	"The AI proposal could not be accepted because a rule or skill did not correctly cite a project file.", "A proposta da IA nao pode ser aceita porque uma regra ou skill nao citou corretamente um arquivo do projeto.",
	"The AI response did not match the expected proposal format.", "A resposta da IA nao seguiu o formato esperado para a proposta.",
	"The AI generated", "A IA gerou",
	"rule(s) and", "regra(s) e",
	"skill(s) without valid project-file citations; these items were discarded. The remaining proposal has verified citations and still requires your review.", "skill(s) sem citacoes validas de arquivos do projeto; esses itens foram descartados. O restante da proposta tem citacoes verificadas e ainda precisa da sua revisao.",
	"Preparing selected context...", "Preparando o contexto selecionado...",
	"Context prepared.", "Contexto preparado.",
	"Waiting for provider response...", "Aguardando a resposta do provedor...",
	"Provider response received.", "Resposta do provedor recebida.",
	"Validating the AI proposal...", "Validando a proposta de IA...",
	"AI proposal ready for review.", "Proposta de IA pronta para revisao.",
	"Cancelling the AI request...", "Cancelando a consulta a IA...",
	"Summary:", "Resumo:",
	"deepseek key found", "Chave do DeepSeek encontrada",
	"openai key found", "Chave do OpenAI encontrada",
	"gemini key found", "Chave do Gemini encontrada",
	"groq key found", "Chave do Groq encontrada",
	"in the environment; it will be used only for this run.", "no ambiente; sera usada apenas nesta execucao.",
	"document(s).", "documento(s).",
	"Project\n", "Projeto\n",
	"Languages\n", "Linguagens\n",
	"Build\n", "Compilacao\n",
	"Project structure\n", "Estrutura do projeto\n",
	"Architecture signals\n", "Indicios de arquitetura\n",
	"Conventions\n", "Convencoes\n",
	"Infrastructure\n", "Infraestrutura\n",
	"Database\n", "Banco de dados\n",
	"Tests\n", "Testes\n",
	"Quality gates\n", "Verificacoes de qualidade\n",
	"Summary\n", "Resumo\n",
	"Files:", "Arquivos:",
	"Detected languages:", "Linguagens detectadas:",
	"Languages [Enter: use detected; 2: adjust or add]:", "Linguagens [Enter: usar detectadas; 2: ajustar ou acrescentar]:",
	"Choose Enter to use detected languages, or 2 to adjust.", "Pressione Enter para usar as linguagens detectadas ou 2 para ajustar.",
	"No programming languages detected; choose languages to guide the harness.", "Nenhuma linguagem de programacao detectada; escolha as linguagens para orientar o harness.",
	"key found in the environment; it will be used only for this run.", "encontrada no ambiente; sera usada apenas nesta execucao.",
	"Models (availability depends on your account or local installation):", "Modelos (disponibilidade depende da sua conta ou instalacao local):",
	"Advanced: enter a model identifier", "Avancado: informar identificador do modelo",
	"Model [1]:", "Modelo [1]:",
	"Model identifier:", "Identificador do modelo:",
	"Choose a model number from 1 to", "Escolha um numero de modelo de 1 a",
	", or 0 for advanced input.", ", ou 0 para entrada avancada.",
	"Context browser:", "Navegador de contexto:",
	"Number: open directory / toggle file; m NUMBER: mark directory; ..: parent; r: review/remove; p: advanced paths; Enter: continue:", "Numero: abrir diretorio / marcar arquivo; m NUMERO: marcar diretorio; ..: voltar; r: revisar/remover; p: caminhos avancados; Enter: continuar:",
	"Remove document number (Enter: return):", "Numero do documento a remover (Enter: voltar):",
	"Choose a document number from the selection.", "Escolha um numero de documento da selecao.",
	"Choose an entry number or a browser command.", "Escolha o numero de um item ou comando do navegador.",
	"Add context files or directories (one path per line; empty line to continue).", "Adicionar arquivos ou diretorios de contexto (um caminho por linha; linha vazia para continuar).",
	"Path:", "Caminho:",
	"Skipped", "Ignorado",
	"Added", "Adicionado(s)",
	"text document(s).", "documento(s) de texto.",
	"Setup cancelled; no project files changed.", "Configuracao cancelada; nenhum arquivo do projeto foi alterado.",
	"This project already has a configured harness. Nothing was changed. Use `harnessforge doctor` to inspect it or `harnessforge generate codex` after reviewing rules.", "Este projeto ja tem um harness configurado. Nenhum arquivo foi alterado. Use `harnessforge doctor` para inspeciona-lo ou `harnessforge generate codex` depois de revisar as regras.",
	"HarnessForge setup", "Configuracao do HarnessForge",
	"Project:", "Projeto:",
	"Selected languages:", "Linguagens selecionadas:",
	"Ready to analyze", "Pronto para analisar",
	"Analyze this project now?", "Analisar este projeto agora?",
	"Analysis cancelled; no project files changed.", "Analise cancelada; nenhum arquivo do projeto foi alterado.",
	"Analyzing project...", "Analisando o projeto...",
	"What HarnessForge understood", "O que o HarnessForge identificou",
	"Languages selected:", "Linguagens selecionadas:",
	"Top-level directories:", "Diretorios na raiz:",
	"No project files have been changed.", "Nenhum arquivo do projeto foi alterado.",
	"Which languages should guide the harness?", "Quais linguagens devem orientar o harness?",
	"Languages [all detected]:", "Linguagens [todas as detectadas]:",
	"Use AI for a tailored setup proposal?", "Usar IA para uma proposta personalizada?",
	"Provider [", "Provedor [",
	"(default ", "(padrao ",
	"Model (", "Modelo (",
	"AI token received for this run; it will not be written to project files.", "Chave de IA disponivel nesta execucao; nao sera gravada nos arquivos do projeto.",
	"Context detected automatically:", "Contexto detectado automaticamente:",
	"Selected context:", "Contexto selecionado:",
	"first 16 KiB only", "somente os primeiros 16 KiB",
	"Additional observations or instructions (one per line; empty line to continue):", "Observacoes ou instrucoes adicionais (uma por linha; linha vazia para continuar):",
	"The selected project analysis,", "A analise selecionada do projeto,",
	"document(s), and your observations will be sent to", "documento(s) e suas observacoes serao enviados para",
	"API keys are never written to project files.", "Chaves de API nunca sao gravadas nos arquivos do projeto.",
	"Send this context to the AI provider?", "Enviar este contexto ao provedor de IA?",
	"AI call cancelled; continuing with a local proposal.", "Consulta a IA cancelada; continuando com uma proposta local.",
	"Preparing the proposal with the selected context...", "Preparando a proposta com o contexto selecionado...",
	"AI proposal could not be validated:", "Nao foi possivel validar a proposta de IA:",
	"Continue with a local proposal?", "Continuar com uma proposta local?",
	"Agent instructions", "Instrucoes dos agentes",
	"3 both]", "3 ambos]",
	"Proposed setup (nothing has been written):", "Configuracao proposta (nenhum arquivo foi gravado):",
	"AI summary:", "Resumo da IA:",
	"Architecture:", "Arquitetura:",
	"Languages:", "Linguagens:",
	"Rules:", "Regras:",
	"skills:", "skills:",
	"context documents:", "documentos de contexto:",
	"append to existing instructions", "acrescentar as instrucoes existentes",
	"replace starter/generated file", "substituir arquivo inicial/gerado",
	"(create)", "(criar)",
	"Create or update exactly these files?", "Criar ou atualizar exatamente estes arquivos?",
	"Setup complete. Review the generated files before committing them.", "Configuracao concluida. Revise os arquivos gerados antes de fazer commit.",
	"[y/N]", "[s/N]",
	"[Y/n]", "[S/n]",
)
