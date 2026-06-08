package openai

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"time"

	"ai-platform/internal/relay/common"
	"ai-platform/internal/relay/helper"
	"ai-platform/pkg/translator"
	pkgexecutor "ai-platform/pkg/executor"
)

// formatToTranslator maps platform relay format to shared translator.Format.
var formatToTranslator = map[translator.Format]translator.Format{
	translator.FormatOpenAI:           translator.FormatOpenAI,
	translator.FormatClaude:           translator.FormatClaude,
	translator.FormatGemini:           translator.FormatGemini,
	translator.FormatOpenAIResponses:  translator.FormatOpenAIResponses,
}

// handleTranslatedResponse processes an upstream OpenAI response and writes it
// in the client's requested format using the shared pkg/executor.
func (a *Adaptor) handleTranslatedResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	clientFormat := info.GetOriginalClientFormat()
	tf := formatToTranslator[clientFormat]

	// OpenAI passthrough — no translation needed.
	if tf == translator.FormatOpenAI {
		if info.IsStream {
			return StreamHandler(ctx, resp, info, writer)
		}
		return a.handleChatNonStreamResponse(ctx, resp, info, writer)
	}

	exec := pkgexecutor.GetByProvider("openai")
	if exec == nil {
		if info.IsStream {
			return StreamHandler(ctx, resp, info, writer)
		}
		return a.handleChatNonStreamResponse(ctx, resp, info, writer)
	}

	if info.IsStream {
		return a.translateStreamResponse(ctx, resp, info, writer, exec)
	}
	return a.translateNonStreamResponse(ctx, resp, info, writer, exec)
}

// translateNonStreamResponse reads the full upstream body, translates it, and writes.
func (a *Adaptor) translateNonStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter, exec pkgexecutor.Executor) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		writer.Write(body)
		return &common.Usage{}, nil
	}

	clientFormat := info.GetOriginalClientFormat()
	translated, err := exec.ConvertResponse(body, translator.FormatOpenAI, clientFormat)
	if err != nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		writer.Write(body)
		return &common.Usage{}, nil
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	writer.Write(translated)

	usage := helper.ExtractUsage(body)
	if usage != nil {
		return &common.Usage{
			PromptTokens:           usage.PromptTokens,
			CompletionTokens:       usage.CompletionTokens,
			TotalTokens:            usage.TotalTokens,
			PromptTokensDetails:    common.DtoTokenDetailsToCommon(usage.PromptTokensDetails),
			CompletionTokenDetails: common.DtoTokenDetailsToCommon(usage.CompletionTokenDetails),
		}, nil
	}
	return &common.Usage{}, nil
}

// translateStreamResponse reads the upstream SSE stream, translates each chunk, and writes.
func (a *Adaptor) translateStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter, exec pkgexecutor.Executor) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		writer.Write(body)
		return &common.Usage{}, nil
	}

	clientFormat := info.GetOriginalClientFormat()
	streamConv, err := exec.NewResponseStream(translator.FormatOpenAI, clientFormat)
	if err != nil || streamConv == nil {
		return StreamHandler(ctx, resp, info, writer)
	}

	helper.SetEventStreamHeaders(writer)
	defer helper.PingTicker(writer, 15*time.Second)()

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var totalUsage common.Usage

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return &totalUsage, nil
		default:
		}

		line := scanner.Bytes()
		chunk, convErr := streamConv.Feed(line)
		if convErr == nil && len(chunk) > 0 {
			writer.Write(chunk)
			if flusher, ok := writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}

	tail, _ := streamConv.End()
	if len(tail) > 0 {
		writer.Write(tail)
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	if u := streamConv.Usage(); u != nil {
		totalUsage = common.Usage{
			PromptTokens:     u.PromptTokens,
			CompletionTokens: u.CompletionTokens,
			TotalTokens:      u.TotalTokens,
		}
	}

	return &totalUsage, nil
}
