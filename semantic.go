package main

import (
	"encoding/json"
	"log"
	"strings"
	"unicode"
	"unicode/utf16"
)

// The token types we highlight. A token's type is its index in this list.
// These are all standard LSP types, so editors color them without any setup.
// Each comment names the syntax/patch.vim group it stands in for.
var tokenTypes = []string{
	"keyword",    // Statement, Conditional, Repeat, Todo: START, IF, wait, TODO...
	"comment",    // Comment
	"string",     // String
	"number",     // Number
	"variable",   // plain names, which patch.vim leaves uncolored
	"operator",   // Operator: and or not ... + - * / =
	"function",   // Function: def name, {name, random, getkey...
	"type",       // Type: drawing, sprites, files, log
	"enumMember", // Constant: assignment, motion, lists (editors color it as a constant)
	"macro",      // PreProc: configuration
	"namespace",  // Include: the whole Include line
	"property",   // SpecialChar: built-ins like _self and _x
}

// Modifiers are bit flags: the first is 1, the second 2, the third 4...
var tokenModifiers = []string{"defaultLibrary"}

const (
	TokKeyword = iota
	TokComment
	TokString
	TokNumber
	TokVariable
	TokOperator
	TokFunction
	TokType
	TokConstant
	TokMacro
	TokNamespace
	TokProperty
)

// Built-in names like _self and _x
const ModDefaultLibrary = 1

// The keyword groups from syntax/patch.vim
var keywordGroups = map[int][]string{
	TokKeyword: {"start", "receive", "trap", "end",
		"if", "loop", "while", "repeat", "elif", "else",
		"endif", "endloop", "endwhile", "endrepeat",
		"label", "stopscripts", "stopall", "delete", "instance", "fork", "jump",
		"callstack", "adopt", "kidnap", "changelayer", "wait", "broadcast", "unicast"},
	TokOperator: {"and", "or", "not", "lower", "upper", "abs", "round",
		"int", "float", "str", "eval",
		"sin", "cos", "tan", "arcsin", "arccos", "arctan"},
	TokFunction: {"return", "random", "angle", "distance", "collide", "maskcollide",
		"getkey", "getattribute", "getglob", "getindex"},
	TokConstant: {"set", "setvar", "setattribute", "setglob", "setindex",
		"setposition", "translate", "move",
		"string", "join", "split", "merge", "append", "remove", "insert", "copy",
		"setmask", "setcollider"},
	TokType: {"setsprite", "updatesprite", "colorshift", "sprite", "canvas",
		"draw", "stamp", "text", "clear", "rect", "ellipse", "polygon", "line",
		"load", "unload", "file", "font", "sound", "music", "log"},
	TokMacro: {"configure", "apply", "caption", "hide_mouse", "window_size",
		"target_framerate", "screen_resolution", "fullscreen"},
}

// Each keyword in lowercase, mapped to its token type
var keywordTypes = makeKeywordTypes()

func makeKeywordTypes() map[string]int {
	types := map[string]int{}
	for tokType, words := range keywordGroups {
		for _, word := range words {
			types[word] = tokType
		}
	}
	return types
}

// Unlike other keywords, these are case-sensitive, and only count inside comments
var todoWords = []string{"TODO", "FIXME", "NOTE"}

const operatorChars = "+-*/%`=<>!^&|~"

type SemanticTokensOptions struct {
	Legend SemanticTokensLegend `json:"legend"`
	Full   bool                 `json:"full"`
}

type SemanticTokensLegend struct {
	TokenTypes     []string `json:"tokenTypes"`
	TokenModifiers []string `json:"tokenModifiers"`
}

type SemanticTokensParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}

type SemanticTokens struct {
	Data []int `json:"data"`
}

// One highlighted span, with an absolute position
type Token struct {
	Line, Start, Length, Type, Mods int
	Text                            string
}

func sendSemanticTokens(msg Message) {
	var params SemanticTokensParams
	err := json.Unmarshal(msg.Params, &params)
	if err != nil {
		log.Println("bad semanticTokens params: ", err)
	}
	tokens := tokenize(documents[params.TextDocument.URI])

	err = send(Response{JSONRPC: "2.0", ID: msg.ID, Result: SemanticTokens{Data: encode(tokens)}})
	if err != nil {
		log.Fatal(err)
	}
}

func isWordChar(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_'
}

// Splits the text into highlighted tokens, top to bottom and left to right.
func tokenize(text string) []Token {
	tokens := []Token{}
	inComment := false

	for lineNum, line := range strings.Split(text, "\n") {
		chars := []rune(strings.TrimRight(line, "\r"))

		// The editor counts columns in UTF-16 units, so col[i] is where character i starts
		col := make([]int, len(chars)+1)
		for i, c := range chars {
			col[i+1] = col[i] + utf16.RuneLen(c)
		}
		add := func(start, end, kind, mods int) {
			tokens = append(tokens, Token{lineNum, col[start], col[end] - col[start], kind, mods, string(chars[start:end])})
		}
		// A comment from start to the end of the line, with TODO/FIXME/NOTE split out
		addComment := func(start int) {
			from := start
			for i := start; i < len(chars); {
				if !isWordChar(chars[i]) {
					i++
					continue
				}
				end := i
				for end < len(chars) && isWordChar(chars[end]) {
					end++
				}
				for _, todo := range todoWords {
					if string(chars[i:end]) == todo {
						if from < i {
							add(from, i, TokComment, 0)
						}
						add(i, end, TokKeyword, 0)
						from = end
					}
				}
				i = end
			}
			if from < len(chars) {
				add(from, len(chars), TokComment, 0)
			}
		}

		// Skip the indentation
		first := 0
		for first < len(chars) && unicode.IsSpace(chars[first]) {
			first++
		}
		if first == len(chars) {
			continue
		}
		rest := string(chars[first:])

		// Comments: '#' must be the first character on the line.
		// '#=' starts a multiline comment that ends on the line containing '=#'.
		if inComment {
			addComment(first)
			if strings.Contains(rest, "=#") {
				inComment = false
			}
			continue
		}
		if strings.HasPrefix(rest, "#=") {
			addComment(first)
			inComment = !strings.Contains(rest[2:], "=#")
			continue
		}
		if strings.HasPrefix(rest, "#") {
			addComment(first)
			continue
		}

		for i := first; i < len(chars); {
			c := chars[i]
			startsNumber := func(j int) bool { return j < len(chars) && unicode.IsDigit(chars[j]) }
			afterWord := i > 0 && isWordChar(chars[i-1])

			switch {
			case c == '"':
				// A string runs to the next quote, or to the end of the line
				end := i + 1
				for end < len(chars) && chars[end] != '"' {
					end++
				}
				if end < len(chars) {
					end++ // include the closing quote
				}
				add(i, end, TokString, 0)
				i = end

			// Numbers like 5, 3.5, .5 and -2, but not the 2 in x2
			case !afterWord && (unicode.IsDigit(c) ||
				(c == '.' && startsNumber(i+1)) ||
				(c == '-' && (startsNumber(i+1) || (i+2 < len(chars) && chars[i+1] == '.' && startsNumber(i+2))))):
				end := i + 1
				for end < len(chars) && (unicode.IsDigit(chars[end]) || chars[end] == '.') {
					end++
				}
				add(i, end, TokNumber, 0)
				i = end

			case strings.ContainsRune(operatorChars, c):
				end := i + 1
				for end < len(chars) && strings.ContainsRune(operatorChars, chars[end]) &&
					!(chars[end] == '-' && startsNumber(end+1)) {
					end++
				}
				add(i, end, TokOperator, 0)
				i = end

			case isWordChar(c):
				end := i
				for end < len(chars) && isWordChar(chars[end]) {
					end++
				}
				word := strings.ToLower(string(chars[i:end]))
				followedBySpace := end < len(chars) && chars[end] == ' '

				if word == "include" && followedBySpace {
					// The whole rest of the line is the included script
					add(i, len(chars), TokNamespace, 0)
					end = len(chars)
				} else if tokType, ok := keywordTypes[word]; ok {
					add(i, end, tokType, 0)
				} else if word == "def" && followedBySpace {
					// def and the function name after it
					add(i, end, TokFunction, 0)
					nameStart := end + 1
					nameEnd := nameStart
					for nameEnd < len(chars) && isWordChar(chars[nameEnd]) {
						nameEnd++
					}
					if nameEnd > nameStart {
						add(nameStart, nameEnd, TokFunction, 0)
					}
					end = nameEnd
				} else if i > 0 && chars[i-1] == '{' {
					// A function call like {random 0 10}
					add(i, end, TokFunction, 0)
				} else if c == '_' {
					add(i, end, TokProperty, ModDefaultLibrary)
				} else {
					add(i, end, TokVariable, 0)
				}
				i = end

			default:
				i++
			}
		}
	}
	return tokens
}

// Converts tokens to the LSP format: 5 numbers per token, with each position
// relative to the previous token.
func encode(tokens []Token) []int {
	data := []int{}
	prevLine, prevStart := 0, 0
	for _, t := range tokens {
		deltaLine := t.Line - prevLine
		deltaStart := t.Start
		if deltaLine == 0 {
			deltaStart = t.Start - prevStart
		}
		data = append(data, deltaLine, deltaStart, t.Length, t.Type, t.Mods)
		prevLine, prevStart = t.Line, t.Start
	}
	return data
}
