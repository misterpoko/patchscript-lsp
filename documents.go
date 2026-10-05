package main

import (
	"encoding/json"
	"log"
)

// The text of every open file, keyed by URI
var documents = map[string]string{}

type DidOpenParams struct {
	TextDocument struct {
		URI  string `json:"uri"`
		Text string `json:"text"`
	} `json:"textDocument"`
}

type DidChangeParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}

type DidCloseParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}

func didOpen(msg Message) {
	var params DidOpenParams
	err := json.Unmarshal(msg.Params, &params)
	if err != nil {
		log.Println("bad didOpen params: ", err)
		return
	}
	documents[params.TextDocument.URI] = params.TextDocument.Text
}

func didChange(msg Message) {
	var params DidChangeParams
	err := json.Unmarshal(msg.Params, &params)
	if err != nil {
		log.Println("bad didChange params: ", err)
		return
	}
	// With full sync, the last change holds the whole new text
	if len(params.ContentChanges) > 0 {
		documents[params.TextDocument.URI] = params.ContentChanges[len(params.ContentChanges)-1].Text
	}
}

func didClose(msg Message) {
	var params DidCloseParams
	err := json.Unmarshal(msg.Params, &params)
	if err != nil {
		log.Println("bad didClose params: ", err)
		return
	}
	delete(documents, params.TextDocument.URI)
}
