package main

import (
	"strings"
)

// Completion item kinds, which pick the icon in the editor's popup
const (
	KindFunction = 3
	KindVariable = 6
	KindKeyword  = 14
)

// Finds the names a script declares: variables, functions, labels and objects.
func scanSymbols(text string) []CompletionItem {
	items := []CompletionItem{}
	seen := map[string]bool{}

	add := func(name string, kind int) {
		key := strings.ToLower(name) // Patchscript ignores case
		if name == "_" || seen[key] {
			return
		}
		seen[key] = true
		items = append(items, CompletionItem{Label: name, Kind: kind})
	}

	inComment := false
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		first := strings.ToLower(fields[0])

		// multiline comments: #= ... =#, ending on whichever line has the =#
		if inComment {
			if strings.Contains(line, "=#") {
				inComment = false
			}
			continue
		}
		if strings.HasPrefix(first, "#=") {
			inComment = !strings.Contains(line[strings.Index(line, "#=")+2:], "=#")
			continue
		}
		if strings.HasPrefix(first, "#") {
			continue
		}

		switch first {
		case "set", "setvar", "setglob", "label":
			if len(fields) >= 2 {
				add(fields[1], KindVariable)
			}
		case "def":
			if len(fields) >= 2 {
				add(fields[1], KindFunction)
				for _, param := range fields[2:] {
					add(param, KindVariable)
				}
			}
		case "setattribute":
			// setattribute name value, or setattribute obj name value
			if len(fields) == 3 {
				add(fields[1], KindVariable)
			} else if len(fields) >= 4 {
				add(fields[2], KindVariable)
			}
		case "instance":
			// instance "script" parent new_obj ...
			if len(fields) >= 4 && !strings.Contains(fields[3], "=") {
				add(fields[3], KindVariable)
			}
		}
	}
	return items
}
