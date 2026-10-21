package main

import (
	"encoding/json"
	"log"
	"strings"
)

type FormattingOptions struct {
	TabSize      int  `json:"tabSize"`
	InsertSpaces bool `json:"insertSpaces"`
}

type DocumentFormattingParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Options FormattingOptions `json:"options"`
}

type DocumentRangeFormattingParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Range   Range             `json:"range"`
	Options FormattingOptions `json:"options"`
}

// Words that open a block, and the word that closes each one
var blockEnds = map[string]string{
	"start":   "end",
	"receive": "end",
	"trap":    "end",
	"def":     "return",
	"if":      "endif",
	"loop":    "endloop",
	"while":   "endwhile",
	"repeat":  "endrepeat",
}

func sendFormatting(msg Message) {
	var params DocumentFormattingParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		sendError(msg, "bad formatting params")
		return
	}
	text := documents[params.TextDocument.URI]
	edits := indentEdits(text, params.Options, 0, strings.Count(text, "\n"))
	if err := send(Response{JSONRPC: "2.0", ID: msg.ID, Result: edits}); err != nil {
		log.Fatal(err)
	}
}

func sendRangeFormatting(msg Message) {
	var params DocumentRangeFormattingParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		sendError(msg, "bad range formatting params")
		return
	}
	// A range ending at the start of a line doesn't include that line
	first, last := params.Range.Start.Line, params.Range.End.Line
	if last > first && params.Range.End.Character == 0 {
		last--
	}
	edits := indentEdits(documents[params.TextDocument.URI], params.Options, first, last)
	if err := send(Response{JSONRPC: "2.0", ID: msg.ID, Result: edits}); err != nil {
		log.Fatal(err)
	}
}

// Edits that reindent lines first to last. The whole file is read so the
// lines get the depth of the blocks around them.
func indentEdits(text string, options FormattingOptions, first, last int) []TextEdit {
	unit := "\t"
	if options.InsertSpaces {
		unit = strings.Repeat(" ", max(options.TabSize, 1))
	}

	lines := strings.Split(text, "\n")
	edits := []TextEdit{}
	for lineNum, depth := range indentDepths(lines) {
		if lineNum < first || lineNum > last || depth < 0 {
			continue
		}
		line := strings.TrimRight(lines[lineNum], "\r")
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]

		want := strings.Repeat(unit, depth)
		if indent == line {
			want = "" // a blank line: drop its whitespace
		}
		if indent != want {
			edits = append(edits, TextEdit{
				Range: Range{
					Start: Position{Line: lineNum, Character: 0},
					End:   Position{Line: lineNum, Character: len(indent)},
				},
				NewText: want,
			})
		}
	}
	return edits
}

// How many levels each line should be indented. Lines inside a multiline
// comment are -1, so they're left as written.
func indentDepths(lines []string) []int {
	depths := make([]int, len(lines))
	open := []string{} // the closing word of each block we're inside
	inComment := false

	for i, line := range lines {
		rest := strings.TrimSpace(line)
		depths[i] = len(open)

		if inComment {
			depths[i] = -1
			if strings.Contains(rest, "=#") {
				inComment = false
			}
			continue
		}
		if strings.HasPrefix(rest, "#=") {
			inComment = !strings.Contains(rest[2:], "=#")
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 || strings.HasPrefix(rest, "#") {
			continue
		}

		word := strings.ToLower(fields[0])
		switch {
		case word == "elif" || word == "else":
			// Lines up with its IF, and the lines after it are inside the IF again
			if len(open) > 0 && open[len(open)-1] == "endif" {
				depths[i]--
			}
		case isCloser(open, word):
			// Close the nearest block it ends, along with any left unclosed inside it
			for open[len(open)-1] != word {
				open = open[:len(open)-1]
			}
			open = open[:len(open)-1]
			depths[i] = len(open)
		case blockEnds[word] != "":
			open = append(open, blockEnds[word])
		}
	}
	return depths
}

// Whether word closes one of the open blocks
func isCloser(open []string, word string) bool {
	for _, closer := range open {
		if closer == word {
			return true
		}
	}
	return false
}
