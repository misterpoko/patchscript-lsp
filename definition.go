package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type DefinitionParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position Position `json:"position"`
}

// Commands that create a name, and which kind of name they create
var declaringCommands = map[string]string{
	"def":          "a function",
	"setglob":      "a global",
	"setattribute": "an attribute",
	"label":        "a label",
	"set":          "a local",
	"setvar":       "a local",
}

// A name being created on some line
type Declaration struct {
	Kind  string // "a function", "a global", ...
	Token Token  // the name itself
}

func sendDefinition(msg Message) {
	var params DefinitionParams
	err := json.Unmarshal(msg.Params, &params)
	if err != nil {
		sendError(msg, "bad definition params")
		return
	}
	uri := params.TextDocument.URI
	text := documents[uri]
	tokens := tokenize(text)
	pos := params.Position

	locations := []Location{}
	if target := tokenAt(tokens, pos); target != nil {
		switch {
		case target.Type == TokNamespace:
			// Include std/colors: the rest of the line after "Include" is the script
			fields := strings.Fields(target.Text)
			if len(fields) >= 2 {
				locations = scriptLocation(uri, fields[1])
			}
		case target.Type == TokString && isInstanceScript(tokens, *target):
			// instance "bullet" ...: the string names the script
			locations = scriptLocation(uri, strings.Trim(target.Text, "\""))
		case isName(*target):
			locations = nameDefinitions(uri, tokens, *target)
		}
	}

	// null tells the editor there's no definition
	var result any
	if len(locations) > 0 {
		result = locations
	}
	err = send(Response{JSONRPC: "2.0", ID: msg.ID, Result: result})
	if err != nil {
		log.Fatal(err)
	}
}

// The token under the cursor, preferring a name when the cursor touches two tokens
func tokenAt(tokens []Token, pos Position) *Token {
	var found *Token
	for i, t := range tokens {
		if t.Line == pos.Line && t.Start <= pos.Character && pos.Character <= t.Start+t.Length {
			found = &tokens[i]
			if isName(t) {
				break
			}
		}
	}
	return found
}

// Where name is defined: every def, setglob, setattribute or LABEL of it in the
// project, or for a local, its first assignment in the block.
func nameDefinitions(uri string, tokens []Token, target Token) []Location {
	locations := []Location{}
	isLocal := true

	for _, f := range projectFiles() {
		fileTokens := tokens
		if f.URI != uri {
			fileTokens = tokenize(f.Text)
		}
		for _, d := range declarations(f.Text, fileTokens) {
			if d.Kind != "a local" && strings.EqualFold(d.Token.Text, target.Text) {
				locations = append(locations, Location{URI: f.URI, Range: tokenRange(d.Token)})
				isLocal = false
			}
		}
	}
	if !isLocal {
		return locations
	}

	// A local: the first time it's assigned in this block
	start, end, ok := blockAround(tokens, target.Line)
	if !ok {
		return locations
	}
	for _, d := range declarations(documents[uri], tokens) {
		if d.Kind == "a local" && d.Token.Line >= start && d.Token.Line <= end &&
			strings.EqualFold(d.Token.Text, target.Text) {
			return []Location{{URI: uri, Range: tokenRange(d.Token)}}
		}
	}
	return locations
}

// Every name created in a file, in order. A line creates a name when its first
// token is one of the declaringCommands.
func declarations(text string, tokens []Token) []Declaration {
	lines := strings.Split(text, "\n")
	decls := []Declaration{}

	for i := 0; i < len(tokens); {
		// The tokens on this line
		end := i
		for end < len(tokens) && tokens[end].Line == tokens[i].Line {
			end++
		}
		line := tokens[i:end]
		i = end

		command := strings.ToLower(line[0].Text)
		kind, ok := declaringCommands[command]
		if !ok || line[0].Type == TokComment || line[0].Type == TokString {
			continue
		}

		// Which word is the name: usually the 2nd, but setattribute obj name value has it 3rd
		fields := strings.Fields(lines[line[0].Line])
		nameIndex := 1
		if command == "setattribute" && len(fields) >= 4 {
			nameIndex = 2
		}
		if nameIndex >= len(fields) {
			continue
		}
		for _, t := range line[1:] {
			if isName(t) && strings.EqualFold(t.Text, fields[nameIndex]) {
				decls = append(decls, Declaration{Kind: kind, Token: t})
				break
			}
		}
	}
	return decls
}

func tokenRange(t Token) Range {
	return Range{
		Start: Position{Line: t.Line, Character: t.Start},
		End:   Position{Line: t.Line, Character: t.Start + t.Length},
	}
}

// Whether a string token is the script name in an instance command
func isInstanceScript(tokens []Token, target Token) bool {
	for _, t := range tokens {
		if t.Line == target.Line {
			// It must be the first string after an instance at the start of the line
			if !strings.EqualFold(t.Text, "instance") {
				return false
			}
			for _, s := range tokens {
				if s.Line == target.Line && s.Type == TokString {
					return s.Start == target.Start
				}
			}
		}
	}
	return false
}

// The location of scripts/<name>.patch, found from the scripts folder the current file is in
func scriptLocation(uri, name string) []Location {
	scriptsDir := ""
	for dir := filepath.Dir(uriToPath(uri)); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if filepath.Base(dir) == "scripts" {
			scriptsDir = dir
			break
		}
	}
	if scriptsDir == "" && rootPath != "" {
		scriptsDir = filepath.Join(rootPath, "scripts")
	}
	if scriptsDir == "" {
		return nil
	}

	path := filepath.Join(scriptsDir, filepath.FromSlash(name)+".patch")
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	return []Location{{URI: pathToURI(path), Range: Range{}}}
}
