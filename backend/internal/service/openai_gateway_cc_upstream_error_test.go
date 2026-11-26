//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const ccRejectedToolError = `{"error":{"type":"invalid_request_error","code":"unsupported_tool","message":"tool schema rejected"}}`
const ccPartialUsageChunk = `data: {"id":"chatcmpl_error","model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"partial"},"finish_reason":null}],"usage":{"prompt_tokens":3,"completion_tokens":2}}` + "\n\n"

func forwardCCErrorFixture(t *testing.T, protocol string, stream bool, upstreamBody, contentType string) (*gin.Context, *httptest.ResponseRecorder, *OpenAIForwardResult, error) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	path, body := "/v1/chat/completions", `{"model":"test-model","messages":[{"role":"user","content":"hello"}],"stream":false}`
	account := rawChatCompletionsTestAccount()
	switch protocol {
	case "anthropic":
		path = "/v1/messages"
		body = `{"model":"test-model","messages":[{"role":"user","content":"hello"}],"max_tokens":16,"stream":false}`
		account = forceChatMessagesFallbackAccount()
	case "responses":
		path = "/v1/responses"
		body = `{"model":"test-model","input":"hello","stream":false}`
		account = forceChatResponsesFallbackAccount()
	}
	if stream {
		body = strings.Replace(body, `"stream":false`, `"stream":true`, 1)
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	headers := http.Header{"X-Request-Id": []string{"rid_cc_error"}, "Retry-After": []string{"7"}}
	if contentType != "" {
		headers.Set("Content-Type", contentType)
	}
	svc := &OpenAIGatewayService{
		cfg: rawChatCompletionsTestConfig(),
		httpUpstream: &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(upstreamBody)),
		}},
	}
	var result *OpenAIForwardResult
	var err error
	switch protocol {
	case "anthropic":
		result, err = svc.forwardAnthropicViaRawChatCompletions(context.Background(), c, account, []byte(body), "")
	case "responses":
		result, err = svc.forwardResponsesViaRawChatCompletions(context.Background(), c, account, []byte(body), time.Now())
	default:
		result, err = svc.forwardAsRawChatCompletions(context.Background(), c, account, []byte(body), "")
	}
	return c, rec, result, err
}

func requireCCFailureWithoutSuccess(t *testing.T, protocol, output string) {
	t.Helper()
	require.NotContains(t, output, "event: message_stop")
	require.NotContains(t, output, "event: response.completed")
	require.NotContains(t, output, `"finish_reason":"stop"`)
	require.NotContains(t, output, "data: [DONE]")
	if protocol == "anthropic" {
		require.Contains(t, output, `"type":"error"`)
	}
}

func TestCCUpstreamErrorBeforeOutput(t *testing.T) {
	for _, protocol := range []string{"anthropic", "responses", "chat"} {
		t.Run(protocol, func(t *testing.T) {
			c, rec, result, err := forwardCCErrorFixture(t, protocol, true, "data: "+ccRejectedToolError+"\n\ndata: [DONE]\n\n", "text/event-stream")
			require.Error(t, err, "an upstream error envelope cannot become an empty successful completion")
			require.Nil(t, result, "failed before output: no successful usage result")
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Contains(t, rec.Body.String(), "tool schema rejected")
			requireCCFailureWithoutSuccess(t, protocol, rec.Body.String())
			events := openAICompatOpsEvents(t, c)
			require.NotEmpty(t, events)
			require.Equal(t, rawChatCompletionsTestAccount().ID, events[len(events)-1].AccountID)
		})
	}
}

func TestCCUpstreamRetryableErrorBeforeOutput(t *testing.T) {
	payload := `{"error":{"type":"rate_limit_error","message":"please retry","code":"rate_limit_exceeded"}}`
	for _, protocol := range []string{"anthropic", "responses", "chat"} {
		t.Run(protocol, func(t *testing.T) {
			c, rec, result, err := forwardCCErrorFixture(t, protocol, true, "data: "+payload+"\n\n", "text/event-stream")
			var failover *UpstreamFailoverError
			require.ErrorAs(t, err, &failover)
			require.Nil(t, result)
			require.Equal(t, http.StatusTooManyRequests, failover.StatusCode)
			require.Equal(t, "7", failover.ResponseHeaders.Get("Retry-After"))
			require.False(t, c.Writer.Written(), "retain the ability to retry before committing output")
			require.Empty(t, rec.Body.String())
		})
	}
}

func TestCCUpstreamErrorAfterOutputPreservesPartialUsage(t *testing.T) {
	payload := `{"error":{"type":"rate_limit_error","message":"generation failed"},"usage":{"prompt_tokens":999,"completion_tokens":999}}`
	for _, protocol := range []string{"anthropic", "responses", "chat"} {
		t.Run(protocol, func(t *testing.T) {
			c, rec, result, err := forwardCCErrorFixture(t, protocol, true, ccPartialUsageChunk+"data: "+payload+"\n\ndata: [DONE]\n\n", "text/event-stream")
			require.Error(t, err)
			var failover *UpstreamFailoverError
			require.False(t, errors.As(err, &failover), "never replay after client output")
			require.NotNil(t, result)
			require.Equal(t, 3, result.Usage.InputTokens)
			require.Equal(t, 2, result.Usage.OutputTokens)
			require.Contains(t, rec.Body.String(), "partial")
			require.Contains(t, rec.Body.String(), "generation failed")
			if protocol == "responses" {
				require.Contains(t, rec.Body.String(), "event: response.failed")
				sequence := int64(0)
				forEachOpenAISSEFrame(rec.Body.String(), func(_ string, data []byte) {
					require.Equal(t, sequence, gjson.GetBytes(data, "sequence_number").Int())
					sequence++
				})
			} else {
				require.Contains(t, rec.Body.String(), "event: error")
			}
			requireCCFailureWithoutSuccess(t, protocol, rec.Body.String())
			require.True(t, IsResponseCommitted(c), "the handler must not append a second terminal error")
			require.NotEmpty(t, openAICompatOpsEvents(t, c))
		})
	}
}

func TestCCHTTP200JSONError(t *testing.T) {
	for _, protocol := range []string{"anthropic", "responses", "chat"} {
		for _, contentType := range []string{"application/json", ""} {
			t.Run(protocol+"/"+contentType, func(t *testing.T) {
				_, rec, result, err := forwardCCErrorFixture(t, protocol, false, ccRejectedToolError, contentType)
				require.Error(t, err)
				require.Nil(t, result)
				require.Equal(t, http.StatusBadRequest, rec.Code)
				require.Contains(t, rec.Body.String(), "tool schema rejected")
				requireCCFailureWithoutSuccess(t, protocol, rec.Body.String())
			})
		}
	}
}

func TestCCStreamingRequestHTTP200JSONError(t *testing.T) {
	for _, protocol := range []string{"anthropic", "responses", "chat"} {
		for _, contentType := range []string{"application/json", "text/event-stream", ""} {
			t.Run(protocol+"/"+contentType, func(t *testing.T) {
				_, rec, result, err := forwardCCErrorFixture(t, protocol, true, "{\n  \"error\": {\"type\":\"invalid_request_error\",\"message\":\"tool schema rejected\"}\n}", contentType)
				require.Error(t, err)
				require.Nil(t, result)
				require.Equal(t, http.StatusBadRequest, rec.Code)
				require.Contains(t, rec.Body.String(), "tool schema rejected")
				requireCCFailureWithoutSuccess(t, protocol, rec.Body.String())
			})
		}
	}
}

type ccEmitBeforeNextRead struct {
	payload string
	emitted *bool
}

func (r *ccEmitBeforeNextRead) Read(p []byte) (int, error) {
	if r.payload != "" {
		n := copy(p, r.payload)
		r.payload = r.payload[n:]
		return n, nil
	}
	if !*r.emitted {
		return 0, errors.New("upstream was read again before the first chunk was emitted")
	}
	return 0, io.EOF
}

func TestCCMislabeledSSEDoesNotBufferUntilEOF(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	emitted := false
	resp := &http.Response{
		Header: http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(&ccEmitBeforeNextRead{
			payload: `data: {"choices":[{"delta":{"content":"first"},"finish_reason":"stop"}]}` + "\n\n",
			emitted: &emitted,
		}),
	}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	scan := svc.scanCCStream(c, resp, "test", "", time.Now(), func(*apicompat.ChatCompletionsChunk) { emitted = true })
	require.NoError(t, scan.Err)
	require.True(t, emitted)
}

func TestCCStreamWithoutTerminalFails(t *testing.T) {
	for _, protocol := range []string{"anthropic", "responses", "chat"} {
		for _, payload := range []string{"", ccPartialUsageChunk} {
			t.Run(protocol+"/"+map[bool]string{true: "empty", false: "partial"}[payload == ""], func(t *testing.T) {
				_, rec, result, err := forwardCCErrorFixture(t, protocol, true, payload, "text/event-stream")
				require.Error(t, err)
				if payload == "" {
					var failover *UpstreamFailoverError
					require.ErrorAs(t, err, &failover)
					require.Nil(t, result)
					require.Empty(t, rec.Body.String())
				} else {
					require.NotNil(t, result)
					require.Equal(t, 3, result.Usage.InputTokens)
					require.Equal(t, 2, result.Usage.OutputTokens)
				}
				require.NotContains(t, rec.Body.String(), "event: message_stop")
				require.NotContains(t, rec.Body.String(), "event: response.completed")
				require.NotContains(t, rec.Body.String(), "data: [DONE]")
			})
		}
	}
}

func TestCCStreamCompatibleTerminalWithoutDone(t *testing.T) {
	for _, protocol := range []string{"anthropic", "responses", "chat"} {
		for _, tail := range []string{
			`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n",
			`data: {"choices":[],"usage":{"prompt_tokens":3,"completion_tokens":2}}` + "\n\n",
		} {
			t.Run(protocol+"/"+gjson.Get(strings.TrimPrefix(tail, "data: "), "choices.0.finish_reason").String(), func(t *testing.T) {
				_, rec, result, err := forwardCCErrorFixture(t, protocol, true, ccPartialUsageChunk+tail, "text/event-stream")
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 3, result.Usage.InputTokens)
				require.Equal(t, 2, result.Usage.OutputTokens)
				require.NotContains(t, rec.Body.String(), "event: error")
				require.NotContains(t, rec.Body.String(), "event: response.failed")
			})
		}
	}
}

func TestCCTopLevelBusinessError(t *testing.T) {
	payload := `{"code":"AIError","message":"AI服务异常","choices":[],"usage":{"prompt_tokens":999,"completion_tokens":999}}`
	for _, protocol := range []string{"anthropic", "responses", "chat"} {
		for _, test := range []struct {
			name        string
			stream      bool
			body        string
			contentType string
			afterOutput bool
		}{
			{name: "JSON", body: payload, contentType: "application/json"},
			{name: "JSON for stream request", stream: true, body: payload, contentType: "application/json"},
			{name: "SSE before output", stream: true, body: "data: " + payload + "\n\ndata: [DONE]\n\n", contentType: "text/event-stream"},
			{name: "SSE after output", stream: true, body: ccPartialUsageChunk + "data: " + payload + "\n\ndata: [DONE]\n\n", contentType: "text/event-stream", afterOutput: true},
		} {
			t.Run(protocol+"/"+test.name, func(t *testing.T) {
				c, rec, result, err := forwardCCErrorFixture(t, protocol, test.stream, test.body, test.contentType)
				require.Error(t, err, "a top-level business failure cannot become an empty completion")
				require.Contains(t, rec.Body.String(), "AI服务异常")
				requireCCFailureWithoutSuccess(t, protocol, rec.Body.String())
				if test.afterOutput {
					require.NotNil(t, result)
					require.Equal(t, 3, result.Usage.InputTokens)
					require.Equal(t, 2, result.Usage.OutputTokens)
				} else {
					require.Nil(t, result)
					require.Equal(t, http.StatusBadGateway, rec.Code)
				}
				require.NotEmpty(t, openAICompatOpsEvents(t, c))
			})
		}
	}
}

func TestCCBufferedResponseWithoutChoicesFails(t *testing.T) {
	for _, protocol := range []string{"anthropic", "responses", "chat"} {
		for _, payload := range []string{`{}`, `{"choices":[]}`, `{"code":0,"message":"Success","choices":[]}`} {
			t.Run(protocol+"/"+payload, func(t *testing.T) {
				c, rec, result, err := forwardCCErrorFixture(t, protocol, false, payload, "application/json")
				require.Error(t, err, "a buffered response must contain at least one choice")
				require.Nil(t, result, "an empty response must not return a successful billable result")
				require.Equal(t, http.StatusBadGateway, rec.Code)
				requireCCFailureWithoutSuccess(t, protocol, rec.Body.String())
				require.NotEmpty(t, openAICompatOpsEvents(t, c))
			})
		}
	}
}

func TestCCSuccessBusinessCodesRemainCompatible(t *testing.T) {
	for _, protocol := range []string{"anthropic", "responses", "chat"} {
		for _, code := range []string{"0", `"0"`, `"Success"`} {
			t.Run(protocol+"/"+code+"/JSON", func(t *testing.T) {
				payload := `{"code":` + code + `,"message":"Success","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`
				_, rec, result, err := forwardCCErrorFixture(t, protocol, false, payload, "application/json")
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, http.StatusOK, rec.Code)
				require.Equal(t, 3, result.Usage.InputTokens)
				require.Equal(t, 2, result.Usage.OutputTokens)
				require.Contains(t, rec.Body.String(), "ok")
			})
			t.Run(protocol+"/"+code+"/usage-only tail", func(t *testing.T) {
				tail := `data: {"code":` + code + `,"message":"Success","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":2}}` + "\n\n"
				_, rec, result, err := forwardCCErrorFixture(t, protocol, true, ccPartialUsageChunk+tail, "text/event-stream")
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 3, result.Usage.InputTokens)
				require.Equal(t, 2, result.Usage.OutputTokens)
				require.NotContains(t, rec.Body.String(), "event: error")
				require.NotContains(t, rec.Body.String(), "event: response.failed")
			})
		}
	}
}
