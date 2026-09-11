package analyzer

import (
	"fmt"
	"io"
)

func Print(writer io.Writer, analysis *Analysis) {
	fmt.Fprintln(writer, "HarnessForge")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Project")
	fmt.Fprintf(writer, "%s\n\n", analysis.Project)

	printSection(writer, "Languages", analysis.Languages)
	printSection(writer, "Build", analysis.Build)
	printSection(writer, "Frameworks", analysis.Frameworks)
	printSection(writer, "Infrastructure", analysis.Infrastructure)
	printSection(writer, "Database", analysis.Database)
	printSection(writer, "Tests", analysis.Tests)
	printSection(writer, "Git", analysis.Git)

	fmt.Fprintln(writer, "Summary")
	fmt.Fprintf(writer, "Files: %d\n", analysis.Files)
}

func printSection(writer io.Writer, title string, findings []Finding) {
	if len(findings) == 0 {
		return
	}

	fmt.Fprintln(writer, title)
	for _, item := range findings {
		fmt.Fprintf(writer, "- %s\n", item.Value)
	}
	fmt.Fprintln(writer)
}
