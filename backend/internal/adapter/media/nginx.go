package media

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"zapping-test-service/internal/domain"
	"zapping-test-service/internal/port"
)

const (
	fetchRetries = 5
	fetchDelay   = 2 * time.Second
)

type NginxClient struct {
	baseURL string
}

func NewNginxClient(baseURL string) port.MediaClient {
	return &NginxClient{baseURL: baseURL}
}

// FetchM3U8 retrieves and parses segment.m3u8 from NGINX, retrying on failure.
// The parsed segment list is written once to the DB; subsequent calls use the DB.
func (c *NginxClient) FetchM3U8(ctx context.Context, path string) ([]domain.Segment, error) {
	url := c.baseURL + "/" + path + "/segment.m3u8"
	var lastErr error
	for i := 0; i < fetchRetries; i++ {
		if i > 0 {
			time.Sleep(fetchDelay)
		}
		segs, err := parseM3U8(ctx, url)
		if err == nil && len(segs) > 0 {
			return segs, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("fetch m3u8 after %d attempts: %w", fetchRetries, lastErr)
}

// StreamSegment proxies a .ts file from NGINX to dst using io.Copy (32 KB buffer).
// The segment is never fully loaded into memory.
func (c *NginxClient) StreamSegment(ctx context.Context, path, name string, dst io.Writer) error {
	url := c.baseURL + "/" + path + "/" + name
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("media server HTTP %d for %s/%s", resp.StatusCode, path, name)
	}
	_, err = io.Copy(dst, resp.Body)
	return err
}

func parseM3U8(ctx context.Context, url string) ([]domain.Segment, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}

	var segs []domain.Segment
	var pendingDur float64
	position := 0
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#EXTINF:") {
			raw := strings.TrimPrefix(line, "#EXTINF:")
			parts := strings.SplitN(raw, ",", 2)
			pendingDur, _ = strconv.ParseFloat(parts[0], 64)
		} else if line != "" && !strings.HasPrefix(line, "#") {
			segs = append(segs, domain.Segment{
				Name:     line,
				Duration: pendingDur,
				Position: position,
			})
			position++
			pendingDur = 0
		}
	}
	return segs, scanner.Err()
}
