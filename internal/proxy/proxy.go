package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/sony/gobreaker"
	"myproject/api-gateway/internal/logger"
)

type ReverseProxy struct {
	client *http.Client
	cb     *gobreaker.CircuitBreaker
	logger *logger.Logger
}

type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

func New(log *logger.Logger, cbEnabled bool, cbSettings gobreaker.Settings) *ReverseProxy {
	var cb *gobreaker.CircuitBreaker
	if cbEnabled {
		cb = gobreaker.NewCircuitBreaker(cbSettings)
	}

	return &ReverseProxy{
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		cb:     cb,
		logger: log,
	}
}

func (p *ReverseProxy) Forward(ctx context.Context, req *http.Request, targetURL string, maxRetries int, timeout int) (*Response, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("не правильный url: %w", err)
	}

	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("не могу прочитать тело: %w", err)
		}
		req.Body.Close()
	}

	isIdempotent := req.Method == http.MethodGet || req.Method == http.MethodHead || req.Method == http.MethodOptions
	if !isIdempotent {
		maxRetries = 1
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		var bodyReader io.Reader
		if len(bodyBytes) > 0 {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		proxyReq, err := http.NewRequestWithContext(ctx, req.Method, target.String()+req.URL.RequestURI(), bodyReader)
		if err != nil {
			return nil, fmt.Errorf("не могу создать запрос: %w", err)
		}

		proxyReq.Header = req.Header.Clone()
		proxyReq.Header.Set("X-Forwarded-For", req.Header.Get("X-Forwarded-For"))
		proxyReq.Header.Set("X-Forwarded-Proto", req.Header.Get("X-Forwarded-Proto"))
		proxyReq.Header.Set("X-Forwarded-Host", req.Host)

		var resp *http.Response
		var doErr error

		if p.cb != nil {
			result, err := p.cb.Execute(func() (interface{}, error) {
				return p.client.Do(proxyReq)
			})
			if err != nil {
				doErr = err
			} else {
				resp = result.(*http.Response)
			}
		} else {
			resp, doErr = p.client.Do(proxyReq)
		}

		if doErr == nil {
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("не могу прочитать ответ: %w", err)
			}

			return &Response{
				StatusCode: resp.StatusCode,
				Headers:    resp.Header,
				Body:       body,
			}, nil
		}

		lastErr = doErr
		p.logger.Warn("попытка упала", "attempt", attempt+1, "max_retries", maxRetries, "error", doErr)

		if attempt < maxRetries-1 {
			backoff := time.Duration(attempt*attempt)*100*time.Millisecond + time.Duration(rand.Intn(50))*time.Millisecond
			time.Sleep(backoff)
		}
	}

	return nil, fmt.Errorf("все ретраи неудачны: %w", lastErr)
}
