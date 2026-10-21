package main

import (
	"log"
	"strings"
)

type Notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

const SeverityWarning = 2

// Sends the warnings for a file. The editor replaces whatever it showed before,
// so an empty list clears them.
func publishDiagnostics(uri string) {
	diagnostics := []Diagnostic{}
	if text, ok := documents[uri]; ok {
		diagnostics = unclosedBlockWarnings(text)
	}
	err := send(Notification{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params:  PublishDiagnosticsParams{URI: uri, Diagnostics: diagnostics},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// A warning on every block opener that has no closing word, like an IF with no ENDIF
func unclosedBlockWarnings(text string) []Diagnostic {
	lines := strings.Split(text, "\n")
	_, unclosed := scanBlocks(lines)

	diagnostics := []Diagnostic{}
	for _, b := range unclosed {
		// Indentation is spaces and tabs, so its length in bytes is its length in columns
		start := len(lines[b.Line]) - len(strings.TrimLeft(lines[b.Line], " \t"))
		diagnostics = append(diagnostics, Diagnostic{
			Range: Range{
				Start: Position{Line: b.Line, Character: start},
				End:   Position{Line: b.Line, Character: start + len(b.Word)},
			},
			Severity: SeverityWarning,
			Source:   "patchscript",
			Message:  b.Word + " is missing its " + b.Closer,
		})
	}
	return diagnostics
}
