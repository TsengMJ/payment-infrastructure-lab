// Package redis is the shared Redis client.
// Authentication does not use it. A later rate limiter can.
package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

// Client is a process-wide Redis connection.
type Client struct {
	rdb *goredis.Client
}

// New parses redisURL and returns a client. The connection is lazy.
func New(redisURL string) (*Client, error) {
	opt, err := goredis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis url: %w", err)
	}
	return &Client{rdb: goredis.NewClient(opt)}, nil
}

// Ping checks that Redis accepts a command.
func (c *Client) Ping(ctx context.Context) error {
	if err := c.rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	return nil
}

// Close releases the connection pool.
func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	if err := c.rdb.Close(); err != nil {
		return fmt.Errorf("redis close: %w", err)
	}
	return nil
}
