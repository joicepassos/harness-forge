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

func (Java) Parse(ctx context.Context, path string, source []byte) ([]domain.Symbol, error) {
	clean, err := stripJava(source)
	if err != nil {
		return nil, err
	}
	var symbols []domain.Symbol
	for _, match := range javaDeclaration.FindAllSubmatchIndex(clean, -1) {
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
	if len(symbols) == 0 && strings.Contains(string(clean), "class ") {
		return nil, fmt.Errorf("parse Java source: malformed declaration")
	}
	return symbols, nil
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
