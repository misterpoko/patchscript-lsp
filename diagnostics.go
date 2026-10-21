package main

import (
	"log"
	"strings"
	"unicode/utf16"
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
		diagnostics = append(diagnostics, unnamedArgumentWarnings(text, projectFunctions())...)
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

// The parameter names of every function defined in the project, keyed by lowercase name
func projectFunctions() map[string][]string {
	funcs := map[string][]string{}
	for _, f := range projectFiles() {
		lines := strings.Split(f.Text, "\n")
		for _, d := range declarations(f.Text, tokenize(f.Text)) {
			name := strings.ToLower(d.Token.Text)
			if d.Kind != "a function" || funcs[name] != nil {
				continue
			}
			params := []string{}
			for _, p := range strings.Fields(lines[d.Token.Line])[2:] {
				param, _, _ := strings.Cut(p, "=") // def f x=0 gives x a default
				params = append(params, param)
			}
			funcs[name] = params
		}
	}
	return funcs
}

// A warning on every argument to a function call that isn't written name=value.
// Patchscript matches arguments by name, so these fail when the script runs.
func unnamedArgumentWarnings(text string, funcs map[string][]string) []Diagnostic {
	diagnostics := []Diagnostic{}
	inComment := false

	for lineNum, line := range strings.Split(text, "\n") {
		rest := strings.TrimSpace(line)
		if inComment {
			if strings.Contains(rest, "=#") {
				inComment = false
			}
			continue
		}
		if strings.HasPrefix(rest, "#=") {
			inComment = !strings.Contains(rest[2:], "=#")
			continue
		}
		if strings.HasPrefix(rest, "#") {
			continue
		}

		chars := []rune(strings.TrimRight(line, "\r"))
		// The editor counts columns in UTF-16 units, so col[i] is where character i starts
		col := make([]int, len(chars)+1)
		for i, c := range chars {
			col[i+1] = col[i] + utf16.RuneLen(c)
		}

		// Checks the call in chars[start:end], which starts with the function's name
		checkCall := func(start, end int) {
			args := splitArgs(chars, start, end)
			if len(args) == 0 {
				return
			}
			name := string(chars[args[0][0]:args[0][1]])
			params, ok := funcs[strings.ToLower(name)]
			if !ok {
				return
			}
			for k, a := range args[1:] {
				arg := string(chars[a[0]:a[1]])
				if isNamedArg(arg) {
					continue
				}
				param := "name"
				if k < len(params) {
					param = params[k]
				}
				diagnostics = append(diagnostics, Diagnostic{
					Range: Range{
						Start: Position{Line: lineNum, Character: col[a[0]]},
						End:   Position{Line: lineNum, Character: col[a[1]]},
					},
					Severity: SeverityWarning,
					Source:   "patchscript",
					Message:  "Arguments to " + name + " need a name, like " + param + "=" + arg,
				})
			}
		}

		// The command that starts the line, then every {name ...} call in it
		checkCall(0, len(chars))
		inString := false
		for i, c := range chars {
			if c == '"' {
				inString = !inString
			}
			if c == '{' && !inString {
				checkCall(i+1, closingBrace(chars, i))
			}
		}
	}
	return diagnostics
}

// Splits chars[start:end] at spaces, keeping strings and anything in (), [] or {}
// together. Returns the start and end of each piece.
func splitArgs(chars []rune, start, end int) [][2]int {
	args := [][2]int{}
	for i := start; i < end; {
		if chars[i] == ' ' || chars[i] == '\t' {
			i++
			continue
		}
		argStart := i
		depth := 0
		inString := false
		for ; i < end; i++ {
			c := chars[i]
			if c == '"' {
				inString = !inString
			} else if inString {
				continue
			} else if strings.ContainsRune("([{", c) {
				depth++
			} else if strings.ContainsRune(")]}", c) {
				depth = max(depth-1, 0)
			} else if (c == ' ' || c == '\t') && depth == 0 {
				break
			}
		}
		args = append(args, [2]int{argStart, i})
	}
	return args
}

// Where the } matching the { at open is, or the end of the line if there isn't one
func closingBrace(chars []rune, open int) int {
	depth := 0
	inString := false
	for i := open; i < len(chars); i++ {
		switch {
		case chars[i] == '"':
			inString = !inString
		case inString:
		case chars[i] == '{':
			depth++
		case chars[i] == '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(chars)
}

// Whether arg is written name=value
func isNamedArg(arg string) bool {
	name, _, found := strings.Cut(arg, "=")
	if !found || name == "" {
		return false
	}
	for _, c := range name {
		if !isWordChar(c) {
			return false
		}
	}
	return true
}
