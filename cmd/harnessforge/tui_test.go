package main

import "testing"

func TestInstallCommandRemainsAnAliasForGuidedSetup(t *testing.T) {
	command := newTUICommand()
	if command.Use != "install [repository]" || len(command.Aliases) != 1 || command.Aliases[0] != "tui" {
		t.Fatalf("unexpected compatibility command: %s %v", command.Use, command.Aliases)
	}
}
