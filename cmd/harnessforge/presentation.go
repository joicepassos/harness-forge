package main

import (
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

type presentation struct {
	colorful  bool
	asciiOnly bool
}

func presentationFor(output io.Writer) presentation {
	p := presentation{asciiOnly: os.Getenv("HARNESSFORGE_ASCII") != "" || os.Getenv("LC_ALL") == "C"}
	file, ok := output.(*os.File)
	if !ok || os.Getenv("NO_COLOR") != "" || os.Getenv("CLICOLOR") == "0" || os.Getenv("TERM") == "dumb" {
		return p
	}
	p.colorful = term.IsTerminal(file.Fd())
	return p
}

func (p presentation) brand() string {
	if !p.colorful {
		return "[HF] HarnessForge"
	}
	if p.asciiOnly {
		return p.paint("[HF] HarnessForge", "#00A8E8", true)
	}
	return p.paint("⚒ HarnessForge", "#00A8E8", true)
}

func (p presentation) heading(s string) string { return p.paint(s, "#00A8E8", true) }
func (p presentation) accent(s string) string  { return p.paint(s, "#FF7A00", true) }

func (p presentation) status(kind, message string) string {
	if !p.colorful {
		return message
	}
	icon, color := "•", "#00A8E8"
	switch kind {
	case "success":
		icon, color = "✓", "#38BFA7"
	case "warning":
		icon, color = "!", "#E5A845"
	case "error":
		icon, color = "✗", "#E36B76"
	}
	if p.asciiOnly {
		switch kind {
		case "success":
			icon = "+"
		case "error":
			icon = "x"
		default:
			icon = "!"
		}
	}
	return p.paint(icon, color, true) + " " + message
}

func (p presentation) paint(s, color string, bold bool) string {
	if !p.colorful {
		return s
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(bold).Render(s)
}

func (p presentation) help(description, usage string) string {
	var result strings.Builder
	result.WriteString(p.brand())
	result.WriteString("\n")
	result.WriteString(description)
	result.WriteString("\n\n")
	for _, line := range strings.Split(usage, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(line, " ") {
			line = p.heading(line)
		} else if strings.HasPrefix(line, "  ") && trimmed != "" && !strings.HasPrefix(trimmed, "-") {
			fields := strings.Fields(trimmed)
			if len(fields) > 1 {
				name := fields[0]
				index := strings.Index(line, name)
				line = line[:index] + p.accent(name) + line[index+len(name):]
			}
		}
		result.WriteString(line)
		result.WriteString("\n")
	}
	return result.String()
}

func (p presentation) analysis(text string) string {
	if !p.colorful {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i == 0 && line == "HarnessForge" {
			lines[i] = p.brand()
			continue
		}
		switch line {
		case "Project", "Languages", "Build", "Frameworks", "Infrastructure", "Database", "Tests", "Git", "Summary":
			lines[i] = p.heading(line)
		}
	}
	return strings.Join(lines, "\n")
}
