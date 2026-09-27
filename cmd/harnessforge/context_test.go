package main

import "testing"

func TestContextExplainExposesOptInBM25Flag(t *testing.T) {
	command := newContextCommand()
	explain, _, err := command.Find([]string{"explain"})
	if err != nil {
		t.Fatal(err)
	}
	flag := explain.Flags().Lookup("bm25")
	if flag == nil {
		t.Fatal("--bm25 flag is not exposed")
	}
	if flag.DefValue != "false" {
		t.Fatalf("default bm25 = %q", flag.DefValue)
	}
}
