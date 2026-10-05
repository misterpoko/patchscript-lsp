package main

import (
	"encoding/json"
	"log"
	"sort"
	"strings"
)

type ReferenceParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position Position `json:"position"`
	Context  struct {
		IncludeDeclaration bool `json:"includeDeclaration"`
	} `json:"context"`
}

func sendReferences(msg Message) {
	var params ReferenceParams
	err := json.Unmarshal(msg.Params, &params)
	if err != nil {
		sendError(msg, "bad references params")
		return
	}
	uri := params.TextDocument.URI
	tokens := tokenize(documents[uri])

	locations := []Location{}
	if target := tokenAt(tokens, params.Position); target != nil && isName(*target) {
		locations = findReferences(uri, tokens, *target, params.Context.IncludeDeclaration)
	}

	// null tells the editor there are no references
	var result any
	if len(locations) > 0 {
		result = locations
	}
	err = send(Response{JSONRPC: "2.0", ID: msg.ID, Result: result})
	if err != nil {
		log.Fatal(err)
	}
}

// Every use of the target name, with the same scope rules as rename: shared names
// (functions, globals, attributes, labels) in every file, locals in their block only.
func findReferences(uri string, tokens []Token, target Token, includeDeclaration bool) []Location {
	files := projectFiles()
	fileTokens := make([][]Token, len(files))
	scope := ""
	for i, f := range files {
		if f.URI == uri {
			fileTokens[i] = tokens
		} else {
			fileTokens[i] = tokenize(f.Text)
		}
		if scope == "" {
			scope = sharedScope(f.Text, fileTokens[i], target.Text)
		}
	}

	// Which lines of which files to search
	inScope := func(fileURI string, t Token) bool { return true }
	start, end := -1, -1
	if scope == "" {
		var ok bool
		start, end, ok = blockAround(tokens, target.Line)
		if !ok {
			return nil
		}
		inScope = func(fileURI string, t Token) bool {
			return fileURI == uri && t.Line >= start && t.Line <= end
		}
	}

	// The spots that create the name, matching what go to definition returns:
	// every def/setglob/setattribute/LABEL for shared names, the first assignment for locals
	isDeclaration := map[string]map[Token]bool{}
	if !includeDeclaration {
		for i, f := range files {
			isDeclaration[f.URI] = map[Token]bool{}
			for _, d := range declarations(f.Text, fileTokens[i]) {
				if !strings.EqualFold(d.Token.Text, target.Text) || !inScope(f.URI, d.Token) {
					continue
				}
				if scope != "" && d.Kind != "a local" {
					isDeclaration[f.URI][d.Token] = true
				} else if scope == "" && d.Kind == "a local" {
					isDeclaration[f.URI][d.Token] = true
					break // only the first assignment
				}
			}
		}
	}

	// The current file first, then the rest by name, so the list is stable
	order := make([]int, len(files))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool {
		fa, fb := files[order[a]], files[order[b]]
		if (fa.URI == uri) != (fb.URI == uri) {
			return fa.URI == uri
		}
		return fa.URI < fb.URI
	})

	locations := []Location{}
	for _, i := range order {
		f := files[i]
		for _, t := range fileTokens[i] {
			if isName(t) && strings.EqualFold(t.Text, target.Text) && inScope(f.URI, t) && !isDeclaration[f.URI][t] {
				locations = append(locations, Location{URI: f.URI, Range: tokenRange(t)})
			}
		}
	}
	return locations
}
