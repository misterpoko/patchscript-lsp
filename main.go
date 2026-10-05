package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)


type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   ServerInfo         `json:"serverInfo"`
}

type ServerCapabilities struct {
	TextDocumentSync int `json:"textDocumentSync"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

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
			log.Fatal("Had an error", err)
		}
		var msg Message
		err = json.Unmarshal(body, &msg)
		if err != nil {
			log.Fatal("Had an error", err)
		}
		processMsg(msg)
	}
}


func processMsg(msg Message) {
	switch (msg.Method) {
	case "initialize":
		sendInit(msg)
	default:
		log.Println(msg.Method)
	}
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

func send(v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

func receive(r *bufio.Reader) ([]byte, error) {
	length := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line) // strips the "\r\n"
		if line == "" {
			break // blank line: the header is done
		}
		if v, ok := strings.CutPrefix(line, "Content-Length: "); ok {
			length, err = strconv.Atoi(v)
			if err != nil {
				return nil, err
			}
		}
	}
	body := make([]byte, length)
	_, err := io.ReadFull(r, body)
	return body, err
}





