package main

import (
	"encoding/json"
	"log"
)

// Every keyword from syntax/patch.vim
var keywords = []string{
	// script hats and blocks, capitalized as they're written in projects/
	"START", "RECEIVE", "TRAP", "END", "Include",
	"IF", "LOOP", "WHILE", "REPEAT", "ELIF", "ELSE",
	"ENDIF", "ENDLOOP", "ENDWHILE", "ENDREPEAT",
	// operators
	"and", "or", "not", "lower", "upper", "abs", "round",
	"int", "float", "str", "eval",
	"sin", "cos", "tan", "arcsin", "arccos", "arctan",
	// functions
	"def", "return", "random", "angle", "distance", "collide",
	"maskcollide", "getkey", "getattribute", "getglob", "getindex",
	// assignment and motion
	"set", "setvar", "setattribute", "setglob", "setindex",
	"setposition", "translate", "move",
	// lists, strings, collision
	"string", "join", "split", "merge", "append", "remove", "insert", "copy",
	"setmask", "setcollider",
	// script control
	"LABEL", "stopscripts", "stopall", "delete", "instance", "fork", "jump",
	"callstack", "adopt", "kidnap", "changelayer", "wait", "broadcast", "unicast",
	// sprites, drawing, files, output
	"setsprite", "updatesprite", "colorshift", "sprite", "canvas",
	"draw", "stamp", "text", "clear", "rect", "ellipse", "polygon", "line",
	"load", "unload", "file", "font", "sound", "music", "log",
	// configuration
	"configure", "apply", "caption", "hide_mouse", "window_size",
	"target_framerate", "screen_resolution", "fullscreen",
}

type CompletionOptions struct{}

type CompletionItem struct {
	Label string `json:"label"`
	Kind  int    `json:"kind"`
}

type CompletionParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}





func sendCompletion(msg Message) {
	log.Println(msg)
	items := []CompletionItem{}
	for _, word := range keywords {
		items = append(items, CompletionItem{Label: word, Kind: KindKeyword})
	}

	// Add the names declared in the file being edited
	var params CompletionParams
	err := json.Unmarshal(msg.Params, &params)
	if err != nil {
		log.Println("bad completion params: ", err)
	} else {
		items = append(items, scanSymbols(documents[params.TextDocument.URI])...)
	}

	err = send(Response{JSONRPC: "2.0", ID: msg.ID, Result: items})
	if err != nil {
		log.Fatal(err)
	}

}
