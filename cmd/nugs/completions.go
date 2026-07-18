package main

// Command adapter for shell completion generation.

import "github.com/jmagar/nugs-cli/internal/completion"

func completionCommand(args []string) error { return completion.Command(args) }
