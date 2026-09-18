package infrastructure

import (
	"context"
	"fmt"
	"harnessforge/internal/symbols/domain"
	"regexp"
	"strings"
)

type Java struct{}

var javaDeclaration = regexp.MustCompile(`(?m)\b(class|interface|record|enum)\s+([A-Za-z_$][A-Za-z0-9_$]*)(?:\s+extends\s+([A-Za-z_$][A-Za-z0-9_$]*))?(?:\s+implements\s+([A-Za-z0-9_$.,\s]+))?\s*\{`)
var javaMethod = regexp.MustCompile(`(?m)(?:^|[;{}])\s*(?:(?:public|protected|private|static|final|abstract|synchronized|native|strictfp|default)\s+)*(?:[A-Za-z_$][A-Za-z0-9_$]*(?:\s*<[^{};()]*>)?(?:\s*\[\])?\s+)([A-Za-z_$][A-Za-z0-9_$]*)\s*\([^{};]*\)\s*(?:throws\s+[A-Za-z0-9_$.,\s]+)?\s*(?:\{|;)`)

func (Java) Parse(ctx context.Context, path string, source []byte) ([]domain.Symbol, error) {
	clean, err := stripJava(source)
	if err != nil {
		return nil, err
	}
	if err := balancedJava(clean); err != nil {
		return nil, err
	}
	var symbols []domain.Symbol
	types := javaDeclaration.FindAllSubmatchIndex(clean, -1)
	for _, match := range types {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		kind := string(clean[match[2]:match[3]])
		name := string(clean[match[4]:match[5]])
		var implemented []string
		if match[6] >= 0 {
			implemented = append(implemented, string(clean[match[6]:match[7]]))
		}
		if match[8] >= 0 {
			for _, item := range strings.Split(string(clean[match[8]:match[9]]), ",") {
				if item = strings.TrimSpace(item); item != "" {
					implemented = append(implemented, item)
				}
			}
		}
		if kind == "class" && len(implemented) > 0 {
			kind = "implementation"
		}
		start := 1 + strings.Count(string(clean[:match[0]]), "\n")
		symbols = append(symbols, domain.Symbol{Kind: kind, Name: name, File: path, StartLine: start, EndLine: start + strings.Count(string(clean[match[0]:match[1]]), "\n"), Implements: implemented})
	}
	for _, match := range javaMethod.FindAllSubmatchIndex(clean, -1) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := string(clean[match[2]:match[3]])
		if !insideJavaType(clean, match[2], types) || isJavaControl(name) {
			continue
		}
		start := 1 + strings.Count(string(clean[:match[2]]), "\n")
		symbols = append(symbols, domain.Symbol{Kind: "method", Name: name, File: path, StartLine: start, EndLine: start + strings.Count(string(clean[match[0]:match[1]]), "\n")})
	}
	if len(symbols) == 0 && strings.Contains(string(clean), "class ") {
		return nil, fmt.Errorf("parse Java source: malformed declaration")
	}
	return symbols, nil
}

func balancedJava(source []byte) error {
	depth := 0
	for _, char := range source {
		switch char {
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return fmt.Errorf("parse Java source: unexpected closing brace")
			}
		}
	}
	if depth != 0 {
		return fmt.Errorf("parse Java source: unclosed brace")
	}
	return nil
}

func insideJavaType(source []byte, position int, types [][]int) bool {
	for _, declaration := range types {
		if declaration[1] > position {
			continue
		}
		depth := 0
		for _, char := range source[declaration[1]:position] {
			switch char {
			case '{':
				depth++
			case '}':
				depth--
			}
		}
		if depth >= 0 {
			return true
		}
	}
	return false
}

func isJavaControl(name string) bool {
	return map[string]bool{"if": true, "for": true, "while": true, "switch": true, "catch": true, "return": true, "new": true}[name]
}

func stripJava(source []byte) ([]byte, error) {
	out := append([]byte(nil), source...)
	var state byte
	for i := 0; i < len(out); i++ {
		switch state {
		case 0:
			if out[i] == '"' || out[i] == '\'' {
				state = out[i]
			} else if i+1 < len(out) && out[i] == '/' && out[i+1] == '/' {
				state, out[i], out[i+1] = 'l', ' ', ' '
				i++
			} else if i+1 < len(out) && out[i] == '/' && out[i+1] == '*' {
				state, out[i], out[i+1] = 'b', ' ', ' '
				i++
			}
		case 'l':
			if out[i] == '\n' {
				state = 0
			} else {
				out[i] = ' '
			}
		case 'b':
			if i+1 < len(out) && out[i] == '*' && out[i+1] == '/' {
				out[i], out[i+1], state = ' ', ' ', 0
				i++
			} else if out[i] != '\n' {
				out[i] = ' '
			}
		default:
			if out[i] == '\\' {
				out[i] = ' '
				if i+1 < len(out) {
					i++
					out[i] = ' '
				}
			} else if out[i] == state {
				out[i], state = ' ', 0
			} else if out[i] != '\n' {
				out[i] = ' '
			}
		}
	}
	if state == 'b' || state == '"' || state == '\'' {
		return nil, fmt.Errorf("parse Java source: unterminated comment or literal")
	}
	return out, nil
}
