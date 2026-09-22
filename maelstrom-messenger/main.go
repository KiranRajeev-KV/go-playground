package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type Node struct {
	NodeID string
	NodeIDs []string
	NextMsgID int
	mu sync.RWMutex
}

type Message struct {
	Src  string         `json:"src"`
	Dest string         `json:"dest"`
	Body map[string]any `json:"body"`
}

func NewNode(id string) *Node {
	return &Node{
		NodeID: id,
		NodeIDs: []string{},
		NextMsgID: 0,
	}
}

func (n *Node) Send() {
	
}

func (n *Node) Reply(msg Message) {
	bodyType, ok := msg.Body["type"].(string)
		if !ok || bodyType == "" {
			fmt.Fprintln(os.Stderr, `invalid message: body.type must be a non-empty string`)
		}

	// fmt.Printf(
	// 	"PARSED: %s|%s|%s\n",
	// 	msg.Src,
	// 	msg.Dest,
	// 	bodyType,
	// )
	
	switch bodyType{
		case "init":
			nodeIds, ok := msg.Body["node_ids"].([]string)
			if !ok {
				fmt.Fprintln(os.Stderr, `invalid message: cannot parse node ids`)
			}

			n.NodeIDs = append(n.NodeIDs, nodeIds...)

			body := make(map[string]any)
			body["type"] = "init_ok"
			body["in_reply_to"] = msg.Body["msg_id"].(string)
			body["msg_id"] = n.NextMsgID
			n.NextMsgID++

			res := Message{
				Src: msg.Dest,
				Dest: msg.Src,
				Body: body,
			}

			fmt.Printf("%v",res)
		default:
		
	}
}

func main() {
	n1 := NewNode("1")
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		var msg Message

		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			fmt.Fprintf(os.Stderr, "invalid JSON: %v\n", err)
			continue
		}

	
		n1.Reply(msg)
		
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "reading stdin: %v\n", err)
	}
}
