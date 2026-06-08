// node_e2e_listen.go — connects a node, then waits for incoming requests
// Usage in terminal 1: go run scripts/node_e2e_listen.go
// Usage in terminal 2: curl -X POST http://localhost:8081/v1/chat/completions \
//   -H "Authorization: Bearer sk-e2e-test-key-12345" \
//   -H "Content-Type: application/json" \
//   -d '{"model":"gpt-4o","messages":[{"role":"user","content":"Hi"}]}'
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var apiBase = "http://localhost:8080"

func main() {
	log.SetFlags(log.Ltime)
	log.Println("=== Node E2E Listener ===")

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	walletAddr := hex.EncodeToString(pub)

	// Register
	msg := []byte("flux-node-register-" + walletAddr)
	sig := ed25519.Sign(priv, msg)
	resp := httpPost(apiBase+"/api/v1/nodes/register", jsonBody(map[string]any{
		"wallet_address": walletAddr, "signature": hex.EncodeToString(sig), "name": "e2e-node",
	}))
	log.Printf("register: %v", resp["code"])

	// Signaling WS
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	ws, _, err := dialer.Dial("ws://localhost:8080/api/v1/nodes/ws", nil)
	must("ws", err)
	defer ws.Close()

	ws.WriteJSON(map[string]string{"type": "auth", "node_id": walletAddr})
	var chal map[string]any
	ws.ReadJSON(&chal)
	nonce, _ := hex.DecodeString(chal["nonce"].(string))
	ws.WriteJSON(map[string]any{"type": "auth_response", "data": hex.EncodeToString(ed25519.Sign(priv, nonce))})
	var ok map[string]any
	ws.ReadJSON(&ok)
	token := ok["session_token"].(string)
	log.Printf("signaling auth: token=%s...", token[:16])

	// Register model
	ws.WriteJSON(map[string]any{
		"type": "register", "data": map[string]any{
			"bindings": []map[string]any{
				{"model_code": "gpt-4o", "key_hash": "kh-e2e-" + walletAddr[:8], "input_price_per_1k": 10, "output_price_per_1k": 30},
			},
		},
	})
	time.Sleep(200 * time.Millisecond)
	log.Println("models: gpt-4o")

	// Data WS
	dataWS, _, err := dialer.Dial("ws://localhost:8080/api/v1/nodes/data", nil)
	must("data ws", err)
	defer dataWS.Close()
	dataWS.WriteJSON(map[string]string{"type": "auth", "token": token})
	dataWS.ReadJSON(&ok)
	log.Printf("data ws: %v", ok["type"])

	// Count
	resp = httpGet(apiBase + "/api/v1/nodes/count")
	log.Printf("node count: %v", resp["data"].(map[string]any)["count"])

	// Listen loop
	log.Println("Listening for incoming requests...")
	log.Println("Run in another terminal:")
	log.Println(`  curl -X POST http://localhost:8081/v1/chat/completions \`)
	log.Println(`    -H "Authorization: Bearer sk-e2e-test-key-12345" \`)
	log.Println(`    -H "Content-Type: application/json" \`)
	log.Println(`    -d '{"model":"gpt-4o","messages":[{"role":"user","content":"Hi"}]}'`)
	log.Println()

	reqCount := 0
	for {
		var msg map[string]any
		dataWS.SetReadDeadline(time.Now().Add(60 * time.Second))
		if err := dataWS.ReadJSON(&msg); err != nil {
			log.Printf("read: %v", err)
			break
		}
		reqType, _ := msg["type"].(string)
		if reqType != "request" {
			log.Printf("unexpected: type=%s", reqType)
			continue
		}
		reqID, _ := msg["request_id"].(string)
		reqCount++
		log.Printf(">>> request #%d: reqID=%s", reqCount, reqID)
		log.Printf("    data: %s", truncateJSON(msg["data"]))

		// Send done
		dataWS.WriteJSON(map[string]any{
			"type": "done", "request_id": reqID,
			"data": map[string]any{
				"id": fmt.Sprintf("chatcmpl-e2e-%d", reqCount), "object": "chat.completion",
				"created": time.Now().Unix(), "model": "gpt-4o",
				"choices": []map[string]any{{
					"index": 0,
					"message": map[string]any{"role": "assistant", "content": fmt.Sprintf("E2E response #%d from node! Hello!", reqCount)},
					"finish_reason": "stop",
				}},
				"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
			},
		})
		log.Printf("<<< sent done #%d", reqCount)
	}
}

func must(label string, err error) {
	if err != nil {
		log.Fatalf("%s: %v", label, err)
	}
}
func jsonBody(v any) io.Reader {
	b, _ := json.Marshal(v)
	return bytes.NewReader(b)
}
func httpPost(url string, body io.Reader) map[string]any {
	resp, err := http.Post(url, "application/json", body)
	must("post", err)
	defer resp.Body.Close()
	var r map[string]any
	json.NewDecoder(resp.Body).Decode(&r)
	return r
}
func httpGet(url string) map[string]any {
	resp, err := http.Get(url)
	must("get", err)
	defer resp.Body.Close()
	var r map[string]any
	json.NewDecoder(resp.Body).Decode(&r)
	return r
}
func truncateJSON(v any) string {
	b, _ := json.Marshal(v)
	s := string(b)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
