package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type Body struct {
	Type      string `json:"type"`
	MsgID     int    `json:"msg_id,omitempty"`
	InReplyTo int    `json:"in_reply_to,omitempty"`
}

type Message struct {
	Src  string `json:"src"`
	Dest string `json:"dest"`
	Body Body   `json:"body"`
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		var msg Message

		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			fmt.Fprintf(os.Stderr, "invalid JSON: %v\n", err)
			continue
		}

		if msg.Body.Type == "" {
			fmt.Fprintln(os.Stderr, `invalid message: body.type is required`)
			continue
		}

		fmt.Printf(
			"PARSED: %s|%s|%s\n",
			msg.Src,
			msg.Dest,
			msg.Body.Type,
		)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "reading stdin: %v\n", err)
	}
}
