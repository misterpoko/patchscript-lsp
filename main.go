package main

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
)

func main() {
	f, err := os.Create("/tmp/patchscript-lsp.log")
	if err != nil {
		panic(err)
	}

	log.SetOutput(f)

	reader := bufio.NewReader(os.Stdin)

	for {
		body, err := receive(reader)
		if err != nil {
			log.Fatal("Had an error recv: ", err)
		}
		var msg Message
		err = json.Unmarshal(body, &msg)
		if err != nil {
			log.Fatal("Had an error on unmarshal", err)
		}
		processMsg(msg)
	}
}


func processMsg(msg Message) {
	switch (msg.Method) {
	case "initialize":
		sendInit(msg)
	case "textDocument/didOpen":
		didOpen(msg)
	case "textDocument/didChange":
		didChange(msg)
	case "textDocument/didClose":
		didClose(msg)
	case "textDocument/completion":
		sendCompletion(msg)
	case "textDocument/semanticTokens/full":
		sendSemanticTokens(msg)
	case "textDocument/rename":
		sendRename(msg)
	case "shutdown":
		send(Response{JSONRPC: "2.0", ID: msg.ID, Result: nil})
	case "exit":
		os.Exit(0)
	default:
		log.Println(msg.Method)
	}
}
