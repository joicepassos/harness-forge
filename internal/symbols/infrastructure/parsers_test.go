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
	source := []byte("package fixture;\n// class Fake {}\nString text = \"interface False {}\";\ninterface Port { void connect(); }\nclass Service implements Port { public void connect() {} private String render(int count) { return \"ok\"; } }\n")
	symbols, err := (Java{}).Parse(context.Background(), "Service.java", source)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 5 || symbols[0].Name != "Port" || symbols[1].Kind != "implementation" || symbols[1].Implements[0] != "Port" || symbols[2].Kind != "method" || symbols[2].Name != "connect" || symbols[4].Name != "render" {
		t.Fatalf("%#v", symbols)
	}
	if _, err := (Java{}).Parse(context.Background(), "Bad.java", []byte("class Broken { String value = \"")); err == nil {
		t.Fatal("unterminated literal accepted")
	}
	if _, err := (Java{}).Parse(context.Background(), "Bad.java", []byte("class Broken { void run() {}")); err == nil {
		t.Fatal("unclosed brace accepted")
	}
}

func TestRegistryRejectsUnsupportedLanguage(t *testing.T) {
	if _, err := (Registry{}).Resolve("python"); err == nil {
		t.Fatal("unsupported language accepted")
	}
}
