package infrastructure

import (
	"context"
	"testing"
)

func TestGoExtractsDeclarationsInsteadOfCommentsAndStrings(t *testing.T) {
	source := []byte("package fixture\n// func Fake() {}\nvar text = `type False interface{}`\ntype Port interface { Run() error }\ntype Service struct{}\nfunc (s *Service) Run() error { return nil }\nfunc Build() {}\n")
	symbols, err := (Go{}).Parse(context.Background(), "service.go", source)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 4 || symbols[0].Kind != "interface" || symbols[1].Kind != "implementation" || symbols[1].Implements[0] != "Port" || symbols[2].Receiver != "Service" || symbols[0].StartLine != 4 {
		t.Fatalf("%#v", symbols)
	}
	for _, symbol := range symbols {
		if symbol.Name == "Fake" || symbol.Name == "False" {
			t.Fatal("text was parsed as a declaration")
		}
	}
	if _, err := (Go{}).Parse(context.Background(), "bad.go", []byte("package fixture\nfunc")); err == nil {
		t.Fatal("syntax error accepted")
	}
}

func TestJavaExtractsDeclarationsInsteadOfCommentsAndStrings(t *testing.T) {
	source := []byte("package fixture;\n// class Fake {}\nnString text = \"interface False {}\";\ninterface Port {}\nclass Service implements Port { }\n")
	symbols, err := (Java{}).Parse(context.Background(), "Service.java", source)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 2 || symbols[0].Name != "Port" || symbols[1].Kind != "implementation" || symbols[1].Implements[0] != "Port" {
		t.Fatalf("%#v", symbols)
	}
	if _, err := (Java{}).Parse(context.Background(), "Bad.java", []byte("class Broken { String value = \"")); err == nil {
		t.Fatal("unterminated literal accepted")
	}
}

func TestRegistryRejectsUnsupportedLanguage(t *testing.T) {
	if _, err := (Registry{}).Resolve("python"); err == nil {
		t.Fatal("unsupported language accepted")
	}
}
