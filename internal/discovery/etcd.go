package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

var ErrNoEndpoints = errors.New("no endpoints found")

type Config struct {
	Endpoints     []string
	DialTimeout   time.Duration
	ServicePrefix string
	LeaseTTL      int64
}

type Client struct {
	cli           *clientv3.Client
	servicePrefix string
	leaseTTL      int64
}

type Endpoint struct {
	Service    string            `json:"service"`
	InstanceID string            `json:"instance_id"`
	Addr       string            `json:"addr"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Key        string            `json:"-"`
}

type endpointValue struct {
	Addr     string            `json:"addr"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Registration struct {
	client          *Client
	key             string
	leaseID         clientv3.LeaseID
	keepAliveCancel context.CancelFunc
}

func New(cfg Config) (*Client, error) {
	if len(cfg.Endpoints) == 0 {
		cfg.Endpoints = []string{"http://127.0.0.1:2379"}
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 5 * time.Second
	}
	if cfg.ServicePrefix == "" {
		cfg.ServicePrefix = "/chatroom/services"
	}
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = 10
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("create etcd client: %w", err)
	}

	return &Client{
		cli:           cli,
		servicePrefix: strings.TrimRight(cfg.ServicePrefix, "/"),
		leaseTTL:      cfg.LeaseTTL,
	}, nil
}

func (c *Client) Close() error {
	if c == nil || c.cli == nil {
		return nil
	}
	return c.cli.Close()
}

func (c *Client) servicePrefixKey(service string) string {
	return c.servicePrefix + "/" + strings.TrimSpace(service) + "/"
}

func (c *Client) serviceKey(service, instanceID string) string {
	return c.servicePrefixKey(service) + strings.TrimSpace(instanceID)
}

func (c *Client) Register(
	ctx context.Context,
	service string,
	instanceID string,
	addr string,
	metadata map[string]string,
) (*Registration, error) {
	service = strings.TrimSpace(service)
	instanceID = strings.TrimSpace(instanceID)
	addr = strings.TrimSpace(addr)

	if service == "" {
		return nil, fmt.Errorf("service is required")
	}
	if instanceID == "" {
		return nil, fmt.Errorf("instanceID is required")
	}
	if addr == "" {
		return nil, fmt.Errorf("addr is required")
	}

	key := c.serviceKey(service, instanceID)

	valBytes, err := json.Marshal(endpointValue{
		Addr:     addr,
		Metadata: metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal endpoint value: %w", err)
	}

	leaseResp, err := c.cli.Grant(ctx, c.leaseTTL)
	if err != nil {
		return nil, fmt.Errorf("grant lease: %w", err)
	}

	if _, err := c.cli.Put(ctx, key, string(valBytes), clientv3.WithLease(leaseResp.ID)); err != nil {
		return nil, fmt.Errorf("put endpoint key: %w", err)
	}

	keepCtx, cancel := context.WithCancel(context.Background())
	ch, err := c.cli.KeepAlive(keepCtx, leaseResp.ID)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("start keepalive: %w", err)
	}

	go func() {
		for range ch {
			// consume keepalive responses until ctx canceled or channel closes
		}
	}()

	return &Registration{
		client:          c,
		key:             key,
		leaseID:         leaseResp.ID,
		keepAliveCancel: cancel,
	}, nil
}

func (r *Registration) Close(ctx context.Context) error {
	if r == nil || r.client == nil {
		return nil
	}

	if r.keepAliveCancel != nil {
		r.keepAliveCancel()
	}

	if r.leaseID != 0 {
		if _, err := r.client.cli.Revoke(ctx, r.leaseID); err != nil {
			return fmt.Errorf("revoke lease: %w", err)
		}
	}

	return nil
}

func (c *Client) ResolveAll(ctx context.Context, service string) ([]Endpoint, error) {
	service = strings.TrimSpace(service)
	if service == "" {
		return nil, fmt.Errorf("service is required")
	}

	prefix := c.servicePrefixKey(service)

	resp, err := c.cli.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("get endpoints by prefix: %w", err)
	}

	out := make([]Endpoint, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		ep, err := parseKV(service, prefix, string(kv.Key), string(kv.Value))
		if err != nil {
			return nil, err
		}
		out = append(out, ep)
	}

	return out, nil
}

func (c *Client) ResolveOne(ctx context.Context, service string) (*Endpoint, error) {
	eps, err := c.ResolveAll(ctx, service)
	if err != nil {
		return nil, err
	}
	if len(eps) == 0 {
		return nil, ErrNoEndpoints
	}

	// 最小版本：先返回第一个
	ep := eps[0]
	return &ep, nil
}

func (c *Client) Watch(ctx context.Context, service string) (<-chan []Endpoint, <-chan error) {
	updates := make(chan []Endpoint, 1)
	errs := make(chan error, 1)

	go func() {
		defer close(updates)
		defer close(errs)

		sendSnapshot := func() bool {
			eps, err := c.ResolveAll(ctx, service)
			if err != nil {
				select {
				case errs <- err:
				default:
				}
				return false
			}
			select {
			case updates <- eps:
				return true
			case <-ctx.Done():
				return false
			}
		}

		if !sendSnapshot() {
			return
		}

		prefix := c.servicePrefixKey(service)
		wch := c.cli.Watch(ctx, prefix, clientv3.WithPrefix())

		for {
			select {
			case <-ctx.Done():
				return
			case wr, ok := <-wch:
				if !ok {
					return
				}
				if err := wr.Err(); err != nil {
					select {
					case errs <- err:
					default:
					}
					continue
				}
				if !sendSnapshot() {
					return
				}
			}
		}
	}()

	return updates, errs
}

func parseKV(service, prefix, key, value string) (Endpoint, error) {
	var val endpointValue
	if err := json.Unmarshal([]byte(value), &val); err != nil {
		return Endpoint{}, fmt.Errorf("unmarshal endpoint value for key %s: %w", key, err)
	}

	instanceID := strings.TrimPrefix(key, prefix)

	return Endpoint{
		Service:    service,
		InstanceID: instanceID,
		Addr:       val.Addr,
		Metadata:   val.Metadata,
		Key:        key,
	}, nil
}
