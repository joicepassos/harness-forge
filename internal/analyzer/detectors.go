package analyzer

import "strings"

func detectLanguages(files []string) []Finding {
	var findings []Finding

	if matches := filesWithExtension(files, ".go"); len(matches) > 0 {
		findings = append(findings, finding("Go", matches...))
	}

	if matches := filesWithExtension(files, ".java"); len(matches) > 0 {
		findings = append(findings, finding("Java", matches...))
	}

	if matches := filesWithExtension(files, ".ts"); len(matches) > 0 {
		findings = append(findings, finding("TypeScript", matches...))
	}

	if matches := filesWithExtension(files, ".js"); len(matches) > 0 {
		findings = append(findings, finding("JavaScript", matches...))
	}

	if matches := filesWithExtension(files, ".py"); len(matches) > 0 {
		findings = append(findings, finding("Python", matches...))
	}

	return findings
}

func detectBuild(files []string) []Finding {
	var findings []Finding

	if hasFile(files, "go.mod") {
		findings = append(findings, finding("Go modules", "go.mod"))
	}

	if hasFile(files, "pom.xml") {
		findings = append(findings, finding("Maven", "pom.xml"))
	}

	if hasFile(files, "build.gradle") || hasFile(files, "build.gradle.kts") {
		findings = append(findings, finding("Gradle", "build.gradle or build.gradle.kts"))
	}

	if hasFile(files, "package.json") {
		findings = append(findings, finding("npm", "package.json"))
	}

	if hasFile(files, "pyproject.toml") {
		findings = append(findings, finding("Python packaging", "pyproject.toml"))
	}

	return findings
}

func detectFrameworks(repositoryPath string) []Finding {
	var findings []Finding

	if fileContains(repositoryPath, "pom.xml", "spring-boot") {
		findings = append(findings, finding("Spring Boot", "pom.xml contains spring-boot"))
	}

	if fileContains(repositoryPath, "package.json", "react") {
		findings = append(findings, finding("React", "package.json contains react"))
	}

	if fileContains(repositoryPath, "package.json", "next") {
		findings = append(findings, finding("Next.js", "package.json contains next"))
	}

	return findings
}

func detectInfrastructure(files []string) []Finding {
	var findings []Finding

	if hasFile(files, "Dockerfile") {
		findings = append(findings, finding("Docker", "Dockerfile"))
	}

	if hasFile(files, "docker-compose.yml") || hasFile(files, "docker-compose.yaml") {
		findings = append(findings, finding("Docker Compose", "docker-compose.yml or docker-compose.yaml"))
	}

	if hasPrefix(files, ".github/workflows/") {
		findings = append(findings, finding("GitHub Actions", ".github/workflows/"))
	}

	return findings
}

func detectDatabase(repositoryPath string, files []string) []Finding {
	var findings []Finding

	if fileContains(repositoryPath, "pom.xml", "postgresql") || fileContains(repositoryPath, "docker-compose.yml", "postgres") || fileContains(repositoryPath, "docker-compose.yaml", "postgres") {
		findings = append(findings, finding("PostgreSQL", "postgresql/postgres found in project files"))
	}

	if fileContains(repositoryPath, "pom.xml", "flyway") || hasPrefix(files, "db/migration/") {
		findings = append(findings, finding("Flyway", "flyway dependency or db/migration directory"))
	}

	return findings
}

func detectTests(files []string) []Finding {
	var findings []Finding

	for _, file := range files {
		lower := strings.ToLower(file)
		if strings.HasSuffix(lower, "_test.go") {
			findings = append(findings, finding("Go tests", file))
			break
		}
	}

	if hasPrefix(files, "tests/") || hasPrefix(files, "test/") {
		findings = append(findings, finding("Test directory", "tests/ or test/"))
	}

	return findings
}
