package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestPingAndClose(t *testing.T) {
	srv := miniredis.RunT(t)
	client, err := New("redis://" + srv.Addr() + "/0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPingFailsWhenRedisIsDown(t *testing.T) {
	srv := miniredis.RunT(t)
	client, err := New("redis://" + srv.Addr() + "/0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err == nil {
		t.Fatal("Ping succeeded after Redis stopped")
	}
}
