package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type Message struct {
	Src  string         `json:"src"`
	Dest string         `json:"dest"`
	Body map[string]any `json:"body"`
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		var msg Message

		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			fmt.Fprintf(os.Stderr, "invalid JSON: %v\n", err)
			continue
		}

		bodyType, ok := msg.Body["type"].(string)
		if !ok || bodyType == "" {
			fmt.Fprintln(os.Stderr, `invalid message: body.type must be a non-empty string`)
			continue
		}

		fmt.Printf(
			"PARSED: %s|%s|%s\n",
			msg.Src,
			msg.Dest,
			bodyType,
		)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "reading stdin: %v\n", err)
	}
}
