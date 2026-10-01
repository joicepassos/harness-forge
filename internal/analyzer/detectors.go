package analyzer

import "context"

type Detector interface {
	Detect(ctx context.Context, repository Repository) []Finding
}

type languageDetector struct{}

func (languageDetector) Detect(_ context.Context, repository Repository) []Finding {
	return applyRules(repository, []Rule{
		extensionRule{value: "Go", extension: ".go"},
		extensionRule{value: "Java", extension: ".java"},
		extensionRule{value: "TypeScript", extension: ".ts"},
		extensionRule{value: "JavaScript", extension: ".js"},
		extensionRule{value: "Python", extension: ".py"},
		extensionRule{value: "Rust", extension: ".rs"},
		extensionRule{value: "C#", extension: ".cs"},
		extensionRule{value: "Kotlin", extension: ".kt"},
		extensionRule{value: "Ruby", extension: ".rb"},
		extensionRule{value: "PHP", extension: ".php"},
		extensionRule{value: "Swift", extension: ".swift"},
	})
}

type buildDetector struct{}

func (buildDetector) Detect(_ context.Context, repository Repository) []Finding {
	return applyRules(repository, []Rule{
		fileRule{value: "Go modules", paths: []string{"go.mod"}},
		fileRule{value: "Maven", paths: []string{"pom.xml"}},
		fileRule{value: "Gradle", paths: []string{"build.gradle", "build.gradle.kts"}},
		fileRule{value: "npm", paths: []string{"package.json"}},
		fileRule{value: "Python packaging", paths: []string{"pyproject.toml"}},
		fileRule{value: "Cargo", paths: []string{"Cargo.toml"}},
		anyRule{rules: []Rule{suffixRule{value: ".NET", suffix: ".csproj"}, suffixRule{value: ".NET", suffix: ".sln"}}},
		fileRule{value: "Bundler", paths: []string{"Gemfile"}},
		fileRule{value: "Composer", paths: []string{"composer.json"}},
		fileRule{value: "pnpm", paths: []string{"pnpm-lock.yaml"}},
		fileRule{value: "Yarn", paths: []string{"yarn.lock"}},
		fileRule{value: "Bun", paths: []string{"bun.lock", "bun.lockb"}},
	})
}

type frameworkDetector struct{}

func (frameworkDetector) Detect(_ context.Context, repository Repository) []Finding {
	return applyRules(repository, []Rule{
		contentRule{value: "Spring Boot", path: "pom.xml", text: "spring-boot"},
		contentRule{value: "Spring Boot", path: "build.gradle", text: "spring-boot"},
		contentRule{value: "Spring Boot", path: "build.gradle.kts", text: "spring-boot"},
		contentRule{value: "React", path: "package.json", text: "react"},
		contentRule{value: "Next.js", path: "package.json", text: "next"},
		contentRule{value: "Vue", path: "package.json", text: "vue"},
		contentRule{value: "Angular", path: "package.json", text: "@angular/core"},
		contentRule{value: "Express", path: "package.json", text: "express"},
		contentRule{value: "NestJS", path: "package.json", text: "@nestjs/core"},
		contentRule{value: "Svelte", path: "package.json", text: "svelte"},
		contentRule{value: "Django", path: "pyproject.toml", text: "django"},
		contentRule{value: "Django", path: "requirements.txt", text: "django"},
		contentRule{value: "FastAPI", path: "pyproject.toml", text: "fastapi"},
		contentRule{value: "FastAPI", path: "requirements.txt", text: "fastapi"},
		contentRule{value: "Flask", path: "pyproject.toml", text: "flask"},
		contentRule{value: "Rails", path: "Gemfile", text: "rails"},
		extensionContentRule{value: "ASP.NET Core", extension: ".csproj", text: "Microsoft.AspNetCore"},
		contentRule{value: "Gin", path: "go.mod", text: "github.com/gin-gonic/gin"},
	})
}

type infrastructureDetector struct{}

func (infrastructureDetector) Detect(_ context.Context, repository Repository) []Finding {
	return applyRules(repository, []Rule{
		fileRule{value: "Docker", paths: []string{"Dockerfile"}},
		fileRule{value: "Docker Compose", paths: []string{"docker-compose.yml", "docker-compose.yaml"}},
		prefixRule{value: "GitHub Actions", prefix: ".github/workflows/"},
	})
}

type databaseDetector struct{}

func (databaseDetector) Detect(_ context.Context, repository Repository) []Finding {
	return applyRules(repository, []Rule{
		anyRule{rules: []Rule{
			contentRule{value: "PostgreSQL", path: "pom.xml", text: "postgresql"},
			contentRule{value: "PostgreSQL", path: "build.gradle", text: "postgresql"},
			contentRule{value: "PostgreSQL", path: "build.gradle.kts", text: "postgresql"},
			contentRule{value: "PostgreSQL", path: "docker-compose.yml", text: "postgres"},
			contentRule{value: "PostgreSQL", path: "docker-compose.yaml", text: "postgres"},
		}},
		anyRule{rules: []Rule{
			contentRule{value: "Flyway", path: "pom.xml", text: "flyway"},
			contentRule{value: "Flyway", path: "build.gradle", text: "flyway"},
			contentRule{value: "Flyway", path: "build.gradle.kts", text: "flyway"},
			prefixRule{value: "Flyway", prefix: "db/migration/"},
		}},
	})
}

type testDetector struct{}

func (testDetector) Detect(_ context.Context, repository Repository) []Finding {
	return applyRules(repository, []Rule{
		suffixRule{value: "Go tests", suffix: "_test.go"},
		suffixRule{value: "Java tests", suffix: "Test.java"},
		prefixRule{value: "Test directory", prefix: "tests/"},
		prefixRule{value: "Test directory", prefix: "test/"},
	})
}

func applyRules(repository Repository, rules []Rule) []Finding {
	findings := make([]Finding, 0, len(rules))

	for _, rule := range rules {
		result, found := rule.Apply(repository)
		if found {
			findings = append(findings, result)
		}
	}

	return uniqueFindings(findings)
}

func uniqueFindings(findings []Finding) []Finding {
	seen := make(map[string]bool)
	unique := make([]Finding, 0, len(findings))

	for _, item := range findings {
		if seen[item.Value] {
			continue
		}

		seen[item.Value] = true
		unique = append(unique, item)
	}

	return unique
}
