package remote

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxResponse = 8 << 20

type Client struct {
	base      *url.URL
	http      *http.Client
	stateFile string
	mu        sync.Mutex
	cookie    *http.Cookie
}

type sessionState struct {
	Server string       `json:"server"`
	Cookie *http.Cookie `json:"cookie"`
}

type Result struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Data    any               `json:"data,omitempty"`
}

type APIError struct {
	Status     int
	Code       string
	RetryAfter string
}

func (e *APIError) Error() string {
	text := fmt.Sprintf("API returned HTTP %d (%s)", e.Status, e.Code)
	if e.RetryAfter != "" {
		text += "; retry after " + e.RetryAfter + " seconds"
	}
	return text
}

func NewClient(server, stateFile string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(server)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("server must be an http(s) origin, for example http://localhost:8080")
	}
	u.Path = ""
	c := &Client{base: u, stateFile: stateFile, http: &http.Client{
		Timeout:       timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}
	if err := c.loadSession(); err != nil {
		return nil, err
	}
	return c, nil
}

func DefaultStateFile(server string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find client configuration: %w", err)
	}
	hash := sha256.Sum256([]byte(strings.TrimRight(server, "/")))
	return filepath.Join(dir, "cloud", fmt.Sprintf("session-%x.json", hash[:8])), nil
}

func (c *Client) loadSession() error {
	if c.stateFile == "" {
		return nil
	}
	f, err := os.OpenFile(c.stateFile, os.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errors.New("open session file: check its permissions")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return errors.New("inspect session file")
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return errors.New("session file must be a regular file with mode 0600")
	}
	var state sessionState
	if err := json.NewDecoder(io.LimitReader(f, 64<<10)).Decode(&state); err != nil {
		return errors.New("invalid session file; remove it and sign in again")
	}
	if state.Server != c.base.String() {
		return errors.New("session file belongs to a different server")
	}
	c.cookie = state.Cookie
	return nil
}

func (c *Client) updateSession(resp *http.Response) error {
	for _, cookie := range resp.Cookies() {
		if cookie.Name != "cloud_session" {
			continue
		}
		return c.saveSession(cookie)
	}
	return nil
}

func (c *Client) saveSession(cookie *http.Cookie) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cookie = cookie
	if cookie.MaxAge < 0 {
		c.cookie = nil
	}
	if c.stateFile == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.stateFile), 0o700); err != nil {
		return errors.New("create session directory")
	}
	data, err := json.Marshal(sessionState{Server: c.base.String(), Cookie: c.cookie})
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	f, err := os.CreateTemp(filepath.Dir(c.stateFile), ".session-*")
	if err != nil {
		return errors.New("create private session file")
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return errors.New("write session file")
	}
	if err := f.Close(); err != nil {
		return errors.New("close session file")
	}
	if err := os.Rename(f.Name(), c.stateFile); err != nil {
		return errors.New("save session file")
	}
	return nil
}

func (c *Client) request(ctx context.Context, method, route string, query url.Values, headers http.Header, body io.Reader) (*http.Response, error) {
	target := *c.base
	target.Path = "/api/v1" + route
	target.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, errors.New("construct API request")
	}
	req.Header = headers.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	req.Header.Set("User-Agent", "cloud-client/1.0")
	c.mu.Lock()
	if c.cookie != nil && (c.cookie.Expires.IsZero() || c.cookie.Expires.After(time.Now())) && (!c.cookie.Secure || c.base.Scheme == "https") {
		req.AddCookie(c.cookie)
	}
	c.mu.Unlock()
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("API request failed; check server, connectivity and timeout")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		var payload struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&payload)
		code := payload.Error.Code
		if code == "" {
			code = "request_failed"
		}
		return nil, &APIError{Status: resp.StatusCode, Code: code, RetryAfter: resp.Header.Get("Retry-After")}
	}
	if err := c.updateSession(resp); err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	return resp, nil
}

func (c *Client) Call(ctx context.Context, op Operation, args map[string]any, output io.Writer) (Result, error) {
	if err := op.Validate(args); err != nil {
		return Result{}, err
	}
	route := op.Path
	query := make(url.Values)
	headers := make(http.Header)
	if op.Tus {
		headers.Set("Tus-Resumable", "1.0.0")
	}
	for _, parameter := range op.Parameters {
		value, exists := args[parameter.Name]
		if !exists {
			continue
		}
		switch parameter.In {
		case "path":
			id := fmt.Sprint(value)
			if !validID(id) {
				return Result{}, errors.New("invalid resource ID")
			}
			route = strings.ReplaceAll(route, "{"+parameter.Name+"}", id)
		case "query":
			if list, ok := value.([]any); ok {
				for _, item := range list {
					query.Add(parameter.Name, parameterText(item))
				}
			} else {
				query.Set(parameter.Name, parameterText(value))
			}
		case "header":
			headers.Set(parameter.Name, parameterText(value))
		}
	}
	var body io.Reader
	if op.ContentType == "application/json" {
		values := make(map[string]any)
		for _, name := range op.BodyFields {
			if value, ok := args[name]; ok {
				values[name] = value
			}
		}
		data, err := json.Marshal(values)
		if err != nil {
			return Result{}, fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(data)
	} else if op.ContentType != "" {
		data, err := base64.StdEncoding.DecodeString(stringValue(args["data"]))
		if err != nil || len(data) > 1<<20 {
			return Result{}, errors.New("data must be base64 containing at most 1 MiB")
		}
		body = bytes.NewReader(data)
	}
	if op.ContentType != "" {
		headers.Set("Content-Type", op.ContentType)
	}
	resp, err := c.request(ctx, op.Method, route, query, headers, body)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	result := Result{Status: resp.StatusCode}
	result.Headers = safeHeaders(resp.Header)
	if op.Binary {
		if output == nil {
			return Result{}, errors.New("binary response requires an output stream")
		}
		n, err := io.Copy(output, resp.Body)
		if err != nil {
			return Result{}, errors.New("download interrupted; destination is incomplete")
		}
		result.Data = map[string]any{"bytes": n, "output": args["output"]}
	} else if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
		if err != nil {
			return Result{}, errors.New("read API response")
		}
		if len(data) > maxResponse {
			return Result{}, errors.New("API response exceeds 8 MiB")
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if err := decoder.Decode(&result.Data); err != nil {
			return Result{}, errors.New("invalid API JSON response")
		}
	}
	return result, nil
}

func safeHeaders(headers http.Header) map[string]string {
	out := make(map[string]string)
	for _, name := range []string{"Location", "Upload-Offset", "Upload-Length", "Upload-Expires", "Tus-Resumable", "Tus-Version", "Tus-Extension", "Tus-Max-Size"} {
		if value := headers.Get(name); value != "" {
			out[name] = value
		}
	}
	return out
}

func validID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, char := range id {
		allowed := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_' || char == '-'
		if !allowed {
			return false
		}
	}
	return true
}

func parameterText(value any) string {
	if n, ok := value.(float64); ok {
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	return fmt.Sprint(value)
}

func headerInt(headers http.Header, name string) (int64, error) {
	n, err := strconv.ParseInt(headers.Get(name), 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid %s response header", name)
	}
	return n, nil
}
