package main

import (
	"encoding/json"
	"log"
)

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   ServerInfo         `json:"serverInfo"`
}

type ServerCapabilities struct {
	CompletionProvider CompletionOptions `json:"completionProvider"`
	TextDocumentSync int `json:"textDocumentSync"`
	SemanticTokensProvider SemanticTokensOptions `json:"semanticTokensProvider"`
	RenameProvider bool `json:"renameProvider"`
	DefinitionProvider bool `json:"definitionProvider"`
	ReferencesProvider bool `json:"referencesProvider"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func sendInit(msg Message) {
	// Remember the project folder, so rename can find the other files
	var params InitializeParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		log.Println("bad initialize params: ", err)
	}
	if params.RootURI != "" {
		rootPath = uriToPath(params.RootURI)
	} else if len(params.WorkspaceFolders) > 0 {
		rootPath = uriToPath(params.WorkspaceFolders[0].URI)
	}
	log.Println("project folder: ", rootPath)

	err := send(Response{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result: InitializeResult{
			Capabilities: ServerCapabilities{
				TextDocumentSync: 1,
				RenameProvider:   true,
				DefinitionProvider: true,
				ReferencesProvider: true,
				SemanticTokensProvider: SemanticTokensOptions{
					Legend: SemanticTokensLegend{TokenTypes: tokenTypes, TokenModifiers: tokenModifiers},
					Full:   true,
				},
			},
			ServerInfo:   ServerInfo{Name: "patchscript-lsp", Version: "0.1"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

}
