package client

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "path"
    "strings"
    "time"

    "github.com/hashicorp/terraform-plugin-log/tflog"

    "scamp/internal/models"
)

const (
    BaseURL     = "https://my.serverscamp.com/coreapi"
    FlavorsEP   = "/flavors"
    SSHKeysEP   = "/ssh-keys"
    LimitsEP    = "/limits"
    ImagesEP    = "/images"
    TasksEP     = "/tasks"
    InstancesEP = "/instances"
)

type Client struct {
    BaseURL string
    APIKey  string
    http    *http.Client
}

func New(baseURL, apiKey string) *Client {
    if baseURL == "" { baseURL = BaseURL }
    return &Client{
        BaseURL: baseURL,
        APIKey:  apiKey,
        http:    &http.Client{
            Timeout: 60 * time.Second,
            // ensure auth headers survive redirects (Go drops them by default on redirect)
            CheckRedirect: func(req *http.Request, via []*http.Request) error {
                if len(via) > 0 {
                    // copy auth headers from the previous request
                    if v := via[0].Header.Get("Authorization"); v != "" {
                        req.Header.Set("Authorization", v)
                    }
                    if v := via[0].Header.Get("X-API-Key"); v != "" {
                        req.Header.Set("X-API-Key", v)
                    }
                    if v := via[0].Header.Get("Accept"); v != "" {
                        req.Header.Set("Accept", v)
                    }
                    if v := via[0].Header.Get("Content-Type"); v != "" {
                        req.Header.Set("Content-Type", v)
                    }
                }
                // let the redirect proceed; http.Client will still cap at 10 redirects
                return nil
            },
        },
    }
}

func (c *Client) buildURL(ep string, q url.Values) (string, error) {
    u, err := url.Parse(c.BaseURL)
    if err != nil { return "", err }
    basePath := strings.TrimSuffix(u.Path, "/")
    if strings.HasPrefix(ep, "/") {
        u.Path = basePath + ep
    } else {
        u.Path = path.Join(basePath, ep)
    }
    if q != nil {
        u.RawQuery = q.Encode()
    }
    return u.String(), nil
}

func (c *Client) doJSON(ctx context.Context, method, fullURL string, payload any) ([]byte, int, http.Header, error) {
    // We'll manually follow redirects so we always keep auth headers.
    const maxRedirects = 20
    currentURL := fullURL
    triedSlash := false
    var bodyBytes []byte
    var err error
    if payload != nil {
        bodyBytes, err = json.Marshal(payload)
        if err != nil {
            return nil, 0, nil, err
        }
    }

    for redirects := 0; redirects <= maxRedirects; redirects++ {
        var body io.Reader
        if bodyBytes != nil {
            body = bytes.NewReader(bodyBytes)
        }

        req, err := http.NewRequestWithContext(ctx, method, currentURL, body)
        if err != nil {
            return nil, 0, nil, err
        }
        tflog.Info(ctx, "HTTP request", map[string]any{"method": method, "url": currentURL})
        if c.APIKey != "" {
            req.Header.Set("X-API-Key", c.APIKey)
            req.Header.Set("Authorization", "Bearer "+c.APIKey)
        }
        if payload != nil {
            req.Header.Set("Content-Type", "application/json")
        }
        req.Header.Set("Accept", "application/json")

        // prevent the client from auto-following redirects; we handle them manually
        hc := *c.http
        hc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
            return http.ErrUseLastResponse
        }

        resp, err := hc.Do(req)
        if err != nil {
            return nil, 0, nil, err
        }
        defer resp.Body.Close()

        // 3xx redirect? resolve and loop
        if resp.StatusCode >= 300 && resp.StatusCode < 400 {
            loc := resp.Header.Get("Location")
            if loc == "" {
                // redirect without Location; treat as error
                return nil, resp.StatusCode, resp.Header, fmt.Errorf("redirect without Location from %s", currentURL)
            }
            // resolve relative against currentURL
            next, err := url.Parse(loc)
            if err != nil {
                return nil, resp.StatusCode, resp.Header, fmt.Errorf("invalid redirect URL %q: %w", loc, err)
            }
            base, err := url.Parse(currentURL)
            if err != nil {
                return nil, resp.StatusCode, resp.Header, err
            }
            nextURL := base.ResolveReference(next).String()
            tflog.Info(ctx, "HTTP redirect", map[string]any{"from": currentURL, "to": nextURL, "status": resp.StatusCode})
            currentURL = nextURL
            // on redirect with a non-GET method, re-try with GET like browsers usually do for 303 See Other
            if resp.StatusCode == http.StatusSeeOther {
                method = http.MethodGet
                payload = nil
                bodyBytes = nil
            }
            continue
        }

        rb, err := io.ReadAll(resp.Body)
        if err != nil {
            return nil, resp.StatusCode, resp.Header, err
        }

        tflog.Info(ctx, "HTTP response", map[string]any{"status": resp.StatusCode, "url": currentURL})

        if resp.StatusCode >= 400 {
            // Small debug log of error body length (avoid dumping secrets)
            if len(rb) > 0 {
                // log only length; body may contain sensitive info
                tflog.Debug(ctx, "HTTP error body", map[string]any{"status": resp.StatusCode, "url": currentURL, "len": len(rb)})
            }

            // If POST got 404/405 on endpoint without trailing slash, retry once with '/'
            if (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed) &&
               strings.EqualFold(method, http.MethodPost) && !strings.HasSuffix(currentURL, "/") && !triedSlash {
                triedSlash = true
                currentURL = currentURL + "/"
                tflog.Info(ctx, "Retrying POST with trailing slash", map[string]any{"url": currentURL})
                // loop again with the same method/payload/bodyBytes
                continue
            }

            var er models.APIError
            _ = json.Unmarshal(rb, &er)
            if er.Message != "" {
                return nil, resp.StatusCode, resp.Header, fmt.Errorf("%s: %s", er.Code, er.Message)
            }
            return nil, resp.StatusCode, resp.Header, fmt.Errorf("http %d: %s", resp.StatusCode, string(rb))
        }

        return rb, resp.StatusCode, resp.Header, nil
    }

    return nil, 0, nil, fmt.Errorf("stopped after too many redirects while requesting %s", currentURL)
}

func (c *Client) GetJSON(ctx context.Context, ep string, q url.Values, out any) error {
    u, err := c.buildURL(ep, q)
    if err != nil { return err }
    b, _, _, err := c.doJSON(ctx, http.MethodGet, u, nil)
    if err != nil { return err }
    return json.Unmarshal(b, out)
}

func (c *Client) PostJSON(ctx context.Context, ep string, payload any, out any) error {
    u, err := c.buildURL(ep, nil)
    if err != nil { return err }
    b, _, _, err := c.doJSON(ctx, http.MethodPost, u, payload)
    if err != nil { return err }
    if out == nil { return nil }
    return json.Unmarshal(b, out)
}

func (c *Client) Delete(ctx context.Context, ep string) error {
    u, err := c.buildURL(ep, nil)
    if err != nil { return err }
    _, _, _, err = c.doJSON(ctx, http.MethodDelete, u, nil)
    return err
}

func (c *Client) PollTask(ctx context.Context, taskID string, interval time.Duration) (*models.TaskResp, error) {
    if interval <= 0 { interval = 2 * time.Second }
    for {
        u, err := c.buildURL(path.Join(TasksEP, taskID), nil)
        if err != nil { return nil, err }
        b, status, _, err := c.doJSON(ctx, http.MethodGet, u, nil)
        if err != nil {
            if status == 404 {
                tflog.Debug(ctx, "task not found yet, retrying", map[string]any{"task_id": taskID})
                time.Sleep(interval)
                continue
            }
            return nil, err
        }
        var tr models.TaskResp
        if err := json.Unmarshal(b, &tr); err != nil { return nil, err }
        st := tr.Data.Status
        if st == "finished" || tr.Data.Error || tr.Data.Progress >= 100 {
            return &tr, nil
        }
        time.Sleep(interval)
    }
}

// PollURL polls arbitrary task status URL that returns models.TaskResp
func (c *Client) PollURL(ctx context.Context, statusURL string, interval time.Duration) (*models.TaskResp, error) {
	if interval <= 0 { interval = 2 * time.Second }

	// If statusURL is relative (e.g. "/tasks/<id>"), resolve it under BaseURL *keeping* its path prefix (e.g. "/coreapi").
	if u, err := url.Parse(statusURL); err == nil && u.Scheme == "" && u.Host == "" {
		base, err := url.Parse(c.BaseURL)
		if err != nil {
			return nil, err
		}
		joined := path.Join(strings.TrimSuffix(base.Path, "/"), strings.TrimPrefix(u.Path, "/"))
		if !strings.HasPrefix(joined, "/") {
			joined = "/" + joined
		}
		base.Path = joined
		statusURL = base.String()
	}

	// redirects are handled inside doJSON

	for {
		b, status, _, err := c.doJSON(ctx, http.MethodGet, statusURL, nil)
		if err != nil {
			if status == 404 {
				time.Sleep(interval)
				continue
			}
			return nil, err
		}
		var tr models.TaskResp
		if err := json.Unmarshal(b, &tr); err != nil { return nil, err }
		st := tr.Data.Status
		if st == "finished" || tr.Data.Error || tr.Data.Progress >= 100 {
			return &tr, nil
		}
		time.Sleep(interval)
	}
}