package main

import (
	"log"
)

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   ServerInfo         `json:"serverInfo"`
}

type ServerCapabilities struct {
	CompletionProvider CompletionOptions `json:"completionProvider"`
	TextDocumentSync int `json:"textDocumentSync"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func sendInit(msg Message) {
	err := send(Response{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: InitializeResult{
			Capabilities: ServerCapabilities{TextDocumentSync: 1},
			ServerInfo:   ServerInfo{Name: "patchscript-lsp", Version: "0.1"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

}
