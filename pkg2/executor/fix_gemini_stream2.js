const fs = require('fs');
let s = fs.readFileSync('gemini.go', 'utf8');
s = s.replace(/\r\n/g, '\n');

const anchor = 'func (e *GeminiExecutor) denormalizeToOpenAI';

const streamHandlers = `// handleStream reads Gemini SSE stream and passthroughs raw data events.
func (e *GeminiExecutor) handleStream(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
\tdefer resp.Body.Close()

\tif resp.StatusCode != http.StatusOK {
\t\tbody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
\t\treturn nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(body))
\t}

\tflusher, ok := writer.(http.Flusher)
\tif !ok {
\t\treturn nil, fmt.Errorf("streaming not supported by response writer")
\t}

\twriter.Header().Set("Content-Type", "text/event-stream")
\twriter.Header().Set("Cache-Control", "no-cache")
\twriter.Header().Set("Connection", "keep-alive")
\twriter.WriteHeader(http.StatusOK)

\tscanner := NewSSEScanner(resp.Body)
\tvar totalUsage *Usage

\tfor scanner.Scan() {
\t\tevent := scanner.Event()
\t\tif event == nil {
\t\t\tcontinue
\t\t}

\t\t// Track usage from final data.
\t\tif bytes.Contains(event.Data, []byte("usageMetadata")) {
\t\t\tu := extractGeminiUsage(event.Data)
\t\t\tif u != nil {
\t\t\t\ttotalUsage = u
\t\t\t}
\t\t}

\t\t_, _ = writer.Write(event.Raw)
\t\tflusher.Flush()
\t}

\tif err := scanner.Err(); err != nil {
\t\tlog.Printf("[executor:gemini] stream scan error: %v", err)
\t}

\treturn totalUsage, nil
}

// handleStreamToClientFormat converts Gemini SSE stream to OpenAI SSE chunks
// (optionally denormalized to ClientFormat via Denormalizer).
func (e *GeminiExecutor) handleStreamToClientFormat(ctx context.Context, resp *http.Response, info *RequestInfo, writer http.ResponseWriter) (*Usage, error) {
\tdefer resp.Body.Close()

\tif resp.StatusCode != http.StatusOK {
\t\tbody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
\t\treturn nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(body))
\t}

\tflusher, ok := writer.(http.Flusher)
\tif !ok {
\t\treturn nil, fmt.Errorf("streaming not supported by response writer")
\t}

\t// Resolve Denormalizer for ClientFormat.
\ttargetFmt := resolveFormat(info.ClientFormat)
\tdn, dnErr := translator.Denormalize(nil, targetFmt, true)
\tif dnErr != nil {
\t\tdn = nil
\t}

\twriter.Header().Set("Cache-Control", "no-cache")
\twriter.Header().Set("Connection", "keep-alive")
\tif dn != nil {
\t\twriter.Header().Set("Content-Type", dn.Header())
\t} else {
\t\twriter.Header().Set("Content-Type", "text/event-stream")
\t}
\twriter.WriteHeader(http.StatusOK)

\tscanner := NewSSEScanner(resp.Body)
\tvar totalUsage *Usage
\tvar modelName string

\tfor scanner.Scan() {
\t\tevent := scanner.Event()
\t\tif event == nil {
\t\t\tcontinue
\t\t}

\t\t// Parse Gemini response.
\t\tvar gr struct {
\t\t\tCandidates []struct {
\t\t\t\tContent struct {
\t\t\t\t\tRole  string \`json:"role"\`
\t\t\t\t\tParts []struct {
\t\t\t\t\t\tText string \`json:"text"\`
\t\t\t\t\t} \`json:"parts"\`
\t\t\t\t} \`json:"content"\`
\t\t\t\tFinishReason string \`json:"finishReason"\`
\t\t\t} \`json:"candidates"\`
\t\t\tUsageMetadata *struct {
\t\t\t\tPromptTokenCount     int \`json:"promptTokenCount"\`
\t\t\t\tCandidatesTokenCount int \`json:"candidatesTokenCount"\`
\t\t\t\tTotalTokenCount      int \`json:"totalTokenCount"\`
\t\t\t} \`json:"usageMetadata"\`
\t\t}
\t\tif err := json.Unmarshal(event.Data, &gr); err != nil {
\t\t\tcontinue
\t\t}

\t\t// Extract text from first candidate.
\t\tcontent := ""
\t\tif len(gr.Candidates) > 0 {
\t\t\tfor _, p := range gr.Candidates[0].Content.Parts {
\t\t\t\tcontent += p.Text
\t\t\t}
\t\t}

\t\t// OpenAI role chunk on first non-empty content.
\t\tif content != "" {
\t\t\tchunk := map[string]any{
\t\t\t\t"choices": []map[string]any{{"index": 0, "delta": map[string]string{"content": content}}},
\t\t\t}
\t\t\twriteStreamChunk(writer, flusher, chunk, dn)
\t\t}

\t\t// Finish reason.
\t\tif len(gr.Candidates) > 0 && gr.Candidates[0].FinishReason != "" {
\t\t\tfr := "stop"
\t\t\tswitch gr.Candidates[0].FinishReason {
\t\t\tcase "MAX_TOKENS":
\t\t\t\tfr = "length"
\t\t\tcase "SAFETY", "BLOCKLIST":
\t\t\t\tfr = "content_filter"
\t\t\t}
\t\t\tchunk := map[string]any{
\t\t\t\t"choices": []map[string]any{{"index": 0, "delta": map[string]any{}, "finish_reason": fr}},
\t\t\t}
\t\t\twriteStreamChunk(writer, flusher, chunk, dn)
\t\t}

\t\t// Usage.
\t\tif gr.UsageMetadata != nil {
\t\t\tum := gr.UsageMetadata
\t\t\tusageMap := map[string]any{
\t\t\t\t"prompt_tokens":     um.PromptTokenCount,
\t\t\t\t"completion_tokens": um.CandidatesTokenCount,
\t\t\t\t"total_tokens":      um.TotalTokenCount,
\t\t\t}
\t\t\ttotalUsage = &Usage{
\t\t\t\tPromptTokens:     um.PromptTokenCount,
\t\t\t\tCompletionTokens: um.CandidatesTokenCount,
\t\t\t\tTotalTokens:      um.TotalTokenCount,
\t\t\t}
\t\t\tusageChunk := map[string]any{"choices": []map[string]any{}, "usage": usageMap}
\t\t\twriteStreamChunk(writer, flusher, usageChunk, dn)
\t\t}
\t}

\t// [DONE] marker for OpenAI-compatible streams.
\t_, _ = writer.Write([]byte("data: [DONE]\\n\\n"))
\tflusher.Flush()

\t// Finalize.
\tfinal, ferr := dn.Finalize()
\tif dn != nil && ferr == nil && len(final) > 0 {
\t\t_, _ = writer.Write(final)
\t\tflusher.Flush()
\t}

\tif err := scanner.Err(); err != nil {
\t\tlog.Printf("[executor:gemini] stream scan error: %v", err)
\t}

\treturn totalUsage, nil
}

`;

if (!s.includes(anchor)) {
    console.log('ANCHOR NOT FOUND');
    process.exit(1);
}
s = s.replace(anchor, streamHandlers + anchor);
fs.writeFileSync('gemini.go', s, 'utf8');
console.log('OK');
