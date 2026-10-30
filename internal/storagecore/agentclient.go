package storagecore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// AgentClient talks to cmd/storage-agent over its Unix socket. An empty socket means no agent: only the
// plain "files" volume exists.
type AgentClient struct{ Socket string }

type AgentVolume struct {
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	Capabilities struct {
		Quota     bool `json:"quota"`
		Snapshots bool `json:"snapshots"`
	} `json:"capabilities"`
}

type AgentUnit struct {
	Path            string `json:"path"`
	ReferencedBytes uint64 `json:"referencedBytes"`
	AvailableBytes  uint64 `json:"availableBytes"`
}

// AgentError carries the agent's stable code (STORAGE_EXISTS, STORAGE_INVALID, ...).
type AgentError struct {
	Status int
	Code   string
}

func (e AgentError) Error() string { return fmt.Sprintf("storage agent %d %s", e.Status, e.Code) }

var ErrNoAgent = errors.New("storage agent not configured")

func (c AgentClient) do(ctx context.Context, method, path string, in, out any) error {
	if c.Socket == "" {
		return ErrNoAgent
	}
	client := http.Client{Timeout: 2 * time.Minute, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", c.Socket)
	}}}
	var body bytes.Buffer
	if in != nil {
		if e := json.NewEncoder(&body).Encode(in); e != nil {
			return e
		}
	}
	req, e := http.NewRequestWithContext(ctx, method, "http://storage-agent"+path, &body)
	if e != nil {
		return e
	}
	res, e := client.Do(req)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		var f struct {
			Code string `json:"code"`
		}
		json.NewDecoder(res.Body).Decode(&f)
		return AgentError{res.StatusCode, f.Code}
	}
	if out != nil {
		return json.NewDecoder(res.Body).Decode(out)
	}
	return nil
}

func (c AgentClient) Volumes(ctx context.Context) ([]AgentVolume, error) {
	var out struct {
		Items []AgentVolume `json:"items"`
	}
	return out.Items, c.do(ctx, "GET", "/v1/volumes", nil, &out)
}

func (c AgentClient) CreateShare(ctx context.Context, volume, key string) (AgentUnit, error) {
	return c.CreateUnit(ctx, volume, "shares", key)
}

func (c AgentClient) Share(ctx context.Context, volume, key string) (AgentUnit, error) {
	return c.Unit(ctx, volume, "shares", key)
}

// CreateUnit creates a unit (class homes or shares); the agent validates names and owns the paths.
func (c AgentClient) CreateUnit(ctx context.Context, volume, class, id string) (AgentUnit, error) {
	var out AgentUnit
	return out, c.do(ctx, "POST", "/v1/volumes/"+volume+"/units", map[string]string{"class": class, "id": id}, &out)
}

func (c AgentClient) Unit(ctx context.Context, volume, class, id string) (AgentUnit, error) {
	var out AgentUnit
	return out, c.do(ctx, "GET", "/v1/volumes/"+volume+"/units/"+class+"/"+id, nil, &out)
}
