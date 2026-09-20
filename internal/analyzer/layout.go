package analyzer

import (
	"sort"
	"strings"
)

// detectLayout reports observable directory structure and cautious architectural
// signals. It does not infer an architectural style from a framework alone.
func detectLayout(files []string) ([]Finding, []Finding) {
	top := map[string][]string{}
	segments := map[string][]string{}
	for _, file := range files {
		parts := strings.Split(file, "/")
		if len(parts) > 1 && !strings.HasPrefix(parts[0], ".") {
			top[parts[0]] = append(top[parts[0]], file)
		}
		for _, part := range parts[:len(parts)-1] {
			segments[part] = append(segments[part], file)
		}
	}
	var names []string
	for name := range top {
		names = append(names, name)
	}
	sort.Strings(names)
	var structure []Finding
	for _, name := range names {
		if len(structure) >= 20 {
			break
		}
		structure = append(structure, finding(name+"/", top[name]...))
	}
	var architecture []Finding
	add := func(label string, names ...string) {
		var evidence []string
		for _, name := range names {
			evidence = append(evidence, segments[name][0])
		}
		architecture = append(architecture, finding(label, evidence...))
	}
	if len(segments["domain"]) > 0 && len(segments["application"]) > 0 && len(segments["infrastructure"]) > 0 {
		add("Layered domain/application/infrastructure directories", "domain", "application", "infrastructure")
	}
	if len(segments["frontend"]) > 0 && len(segments["backend"]) > 0 {
		add("Separate frontend and backend directories", "frontend", "backend")
	} else if len(segments["client"]) > 0 && len(segments["server"]) > 0 {
		add("Separate client and server directories", "client", "server")
	}
	if len(segments["apps"]) > 0 && len(segments["packages"]) > 0 {
		add("Apps and packages workspace layout", "apps", "packages")
	}
	return structure, architecture
}

func detectConventions(files []string) []Finding {
	var findings []Finding
	var tests, docs, migrations, editor []string
	for _, file := range files {
		lower := strings.ToLower(file)
		if strings.HasSuffix(file, "_test.go") {
			tests = append(tests, file)
		}
		if strings.HasPrefix(lower, "docs/adr/") || strings.HasPrefix(lower, "docs/decisions/") {
			docs = append(docs, file)
		}
		if strings.Contains(lower, "/migration/") || strings.Contains(lower, "/migrations/") {
			migrations = append(migrations, file)
		}
		if lower == ".editorconfig" {
			editor = append(editor, file)
		}
	}
	if len(tests) > 0 {
		findings = append(findings, finding("Go tests use _test.go files", tests...))
	}
	if len(docs) > 0 {
		findings = append(findings, finding("Architecture decision records", docs...))
	}
	if len(migrations) > 0 {
		findings = append(findings, finding("Versioned migration directory", migrations...))
	}
	if len(editor) > 0 {
		findings = append(findings, finding("EditorConfig conventions", editor...))
	}
	return findings
}
