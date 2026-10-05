package main

import (
	"encoding/json"
	"log"
	"path/filepath"
	"strings"
)

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes"` // URI → edits in that file
}

type RenameParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position Position `json:"position"`
	NewName  string   `json:"newName"`
}

type ErrorResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   ResponseError   `json:"error"`
}

type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// The LSP error code for a request that was understood but couldn't be done
const RequestFailed = -32803

func sendError(msg Message, message string) {
	err := send(ErrorResponse{JSONRPC: "2.0", ID: msg.ID, Error: ResponseError{Code: RequestFailed, Message: message}})
	if err != nil {
		log.Fatal(err)
	}
}

// Renames the name under the cursor. Locals only exist inside one
// START/RECEIVE/TRAP ... END block, so they're renamed only there. Functions,
// globals, attributes and labels are shared, so they're renamed in every file.
func sendRename(msg Message) {
	var params RenameParams
	err := json.Unmarshal(msg.Params, &params)
	if err != nil {
		sendError(msg, "bad rename params")
		return
	}
	uri := params.TextDocument.URI
	text := documents[uri]
	tokens := tokenize(text)
	pos := params.Position

	// The name under the cursor
	var target *Token
	for i, t := range tokens {
		if t.Line == pos.Line && t.Start <= pos.Character && pos.Character <= t.Start+t.Length {
			target = &tokens[i]
			if isName(t) {
				break // prefer a name when the cursor touches two tokens
			}
		}
	}
	if target == nil || !isName(*target) {
		sendError(msg, "Only variables and functions can be renamed")
		return
	}
	oldName := target.Text
	if problem := checkNewName(params.NewName); problem != "" {
		sendError(msg, problem)
		return
	}

	// Tokenize every project file once
	files := projectFiles()
	fileTokens := make([][]Token, len(files))
	for i, f := range files {
		fileTokens[i] = tokenize(f.Text)
	}

	// Is the name shared beyond one block anywhere in the project?
	scope := ""
	for i, f := range files {
		if scope == "" {
			scope = sharedScope(f.Text, fileTokens[i], oldName)
		}
	}

	changes := map[string][]TextEdit{}
	if scope == "" {
		// A local: rename it in this block only
		start, end, ok := blockAround(tokens, pos.Line)
		if !ok {
			sendError(msg, "Not inside a START, RECEIVE or TRAP block")
			return
		}
		inBlock := func(t Token) bool { return t.Line >= start && t.Line <= end }
		if conflict(tokens, inBlock, oldName, params.NewName) {
			sendError(msg, params.NewName+" is already used in this block")
			return
		}
		changes[uri] = renameEdits(tokens, inBlock, oldName, params.NewName)
	} else {
		// Shared: rename it in every file
		everywhere := func(t Token) bool { return true }
		for i, f := range files {
			if conflict(fileTokens[i], everywhere, oldName, params.NewName) {
				sendError(msg, params.NewName+" is already used in "+filepath.Base(uriToPath(f.URI)))
				return
			}
		}
		for i, f := range files {
			if edits := renameEdits(fileTokens[i], everywhere, oldName, params.NewName); len(edits) > 0 {
				changes[f.URI] = edits
			}
		}
	}
	log.Printf("rename %s -> %s (%s): %d files", oldName, params.NewName, orLocal(scope), len(changes))

	err = send(Response{JSONRPC: "2.0", ID: msg.ID, Result: WorkspaceEdit{Changes: changes}})
	if err != nil {
		log.Fatal(err)
	}
}

func orLocal(scope string) string {
	if scope == "" {
		return "a local"
	}
	return scope
}

// Variables and functions are renamable; built-ins like _self are properties, so they aren't.
func isName(t Token) bool {
	return t.Type == TokVariable || (t.Type == TokFunction && !strings.EqualFold(t.Text, "def") && keywordTypes[strings.ToLower(t.Text)] != TokFunction)
}

// An edit for every use of oldName in the tokens that pass the filter
func renameEdits(tokens []Token, include func(Token) bool, oldName, newName string) []TextEdit {
	edits := []TextEdit{}
	for _, t := range tokens {
		if include(t) && isName(t) && strings.EqualFold(t.Text, oldName) {
			edits = append(edits, TextEdit{
				Range: Range{
					Start: Position{Line: t.Line, Character: t.Start},
					End:   Position{Line: t.Line, Character: t.Start + t.Length},
				},
				NewText: newName,
			})
		}
	}
	return edits
}

// Patchscript ignores case, so renaming to a different name that already exists would merge them
func conflict(tokens []Token, include func(Token) bool, oldName, newName string) bool {
	if strings.EqualFold(oldName, newName) {
		return false // only the capitalization is changing
	}
	for _, t := range tokens {
		if include(t) && isName(t) && strings.EqualFold(t.Text, newName) {
			return true
		}
	}
	return false
}

// Finds the lines of the hat (START, RECEIVE, TRAP) and END around line.
// Keyword tokens are never inside comments or strings, so they're safe to search.
func blockAround(tokens []Token, line int) (start, end int, ok bool) {
	start, end = -1, -1
	for _, t := range tokens {
		if t.Type != TokKeyword {
			continue
		}
		word := strings.ToLower(t.Text)
		isHat := word == "start" || word == "receive" || word == "trap"

		if t.Line <= line {
			// Before the cursor: remember the last hat, forget it if its block ended
			if isHat {
				start = t.Line
			} else if word == "end" && t.Line < line {
				start = -1
			}
		} else if isHat && end == -1 {
			return -1, -1, false // a new block starts before this one ends
		}
		if word == "end" && t.Line >= line && end == -1 {
			end = t.Line
		}
	}
	return start, end, start != -1 && end != -1
}

// Reports whether name is ever made a function, global, attribute or label in this file.
// Those are shared beyond one block. Returns "" if it isn't.
func sharedScope(text string, tokens []Token, name string) string {
	lines := strings.Split(text, "\n")
	for i, t := range tokens {
		// def name: the tokenizer marks both words as functions
		if t.Type == TokFunction && strings.EqualFold(t.Text, "def") && i+1 < len(tokens) &&
			tokens[i+1].Line == t.Line && strings.EqualFold(tokens[i+1].Text, name) {
			return "a function"
		}
		// Only look at commands: the first token on a line. Comment and string
		// tokens include their # or quotes, so they never match a command below.
		if i > 0 && tokens[i-1].Line == t.Line {
			continue
		}
		fields := strings.Fields(lines[t.Line])
		switch strings.ToLower(t.Text) {
		case "setglob", "getglob":
			if len(fields) >= 2 && strings.EqualFold(fields[1], name) {
				return "a global"
			}
		case "label":
			if len(fields) >= 2 && strings.EqualFold(fields[1], name) {
				return "a label"
			}
		case "setattribute":
			// setattribute name value, or setattribute obj name value
			if (len(fields) == 3 && strings.EqualFold(fields[1], name)) ||
				(len(fields) >= 4 && strings.EqualFold(fields[2], name)) {
				return "an attribute"
			}
		}
	}
	return ""
}

// Returns why newName can't be used as a name, or "" if it's fine.
func checkNewName(newName string) string {
	if newName == "" {
		return "The new name is empty"
	}
	for i, c := range newName {
		if !isWordChar(c) || (i == 0 && c >= '0' && c <= '9') {
			return newName + " isn't a valid name: use letters, digits and _, not starting with a digit"
		}
	}
	if strings.HasPrefix(newName, "_") {
		return "Names starting with _ are reserved for built-ins"
	}
	lower := strings.ToLower(newName)
	if _, isKeyword := keywordTypes[lower]; isKeyword || lower == "def" || lower == "include" {
		return newName + " is a keyword"
	}
	return ""
}
