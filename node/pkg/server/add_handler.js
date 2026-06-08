const fs = require('fs');
let s = fs.readFileSync('node/pkg/server/server.go', 'utf8');

// Fix extra indent on ClientFormat line in handleEmbeddings
s = s.replace('\t\t\tClientFormat:', '\t\t\tClientFormat:');

// Insert handleResponses between handleEmbeddings and handleModels
const marker = '\tprovider.ExecuteWithWriter(r.Context(), body, hash, info, w)\n}\n\nfunc (s *Server) handleModels';
const handlerInsert = `\tprovider.ExecuteWithWriter(r.Context(), body, hash, info, w)
}

func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Model           string          \`json:"model"\`
		Input           json.RawMessage \`json:"input"\`
		Instructions    json.RawMessage \`json:"instructions"\`
		MaxOutputTokens int             \`json:"max_output_tokens,omitempty"\`
		Temperature     float64         \`json:"temperature,omitempty"\`
		TopP            float64         \`json:"top_p,omitempty"\`
		Stream          bool            \`json:"stream,omitempty"\`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, \`{"error":"bad request"}\`, http.StatusBadRequest)
		return
	}

	hash, err := s.router.Pick(req.Model)
	if err != nil {
		http.Error(w, \`{"error":"\`+err.Error()+\`"}\`, http.StatusNotFound)
		return
	}

	info := &executor.RequestInfo{
		RequestID:     r.Header.Get("X-Request-Id"),
		RelayMode:     model.RelayModeChatCompletions,
		IsStream:      req.Stream,
		Model:         req.Model,
		InboundFormat: "openai_responses",
		ClientFormat:  clientFormatFromHeader(r, "openai_responses"),
		StartTime:     time.Now(),
	}

	body, _ := json.Marshal(req)
	provider.ExecuteWithWriter(r.Context(), body, hash, info, w)
}

func (s *Server) handleModels`;

if (s.includes(marker)) {
  s = s.replace(marker, handlerInsert);
  fs.writeFileSync('node/pkg/server/server.go', s);
  console.log('OK');
} else {
  console.log('MARKER NOT FOUND');
  // print the exact bytes around the area
  const idx = s.indexOf('handleModels');
  console.log('context:', JSON.stringify(s.substring(idx - 200, idx)));
}
