//go:build integration

package testutil

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	atlasImage     = "payment-lab-atlas:1.3.0"
	postgresImage  = "postgres:18.6"
	templateDBName = "template"
)

// Postgres is one throwaway server for a test package.
// Each test gets its own database cloned from template after migrations.
type Postgres struct {
	mu       sync.Mutex
	adminDSN string
	close    func(context.Context) error
}

// Start boots one Postgres and applies the merchant and payment migrations
// to a template database. Tests clone that template with Database.
func Start(ctx context.Context) (*Postgres, error) {
	ctr, err := postgres.Run(ctx, postgresImage,
		postgres.WithUsername("payment"),
		postgres.WithPassword("payment"),
		postgres.WithDatabase("payment"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Postgres, error) {
		_ = ctr.Terminate(ctx)
		return nil, err
	}
	admin, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fail(err)
	}
	admin, err = forceLoopback(admin)
	if err != nil {
		return fail(err)
	}
	templateDSN, err := withDatabase(admin, templateDBName)
	if err != nil {
		return fail(err)
	}
	if err := execSQL(ctx, admin, "CREATE DATABASE "+templateDBName); err != nil {
		return fail(err)
	}
	root, err := repoRoot()
	if err != nil {
		return fail(err)
	}
	for _, env := range []string{"merchant", "payment"} {
		if err := applyAtlas(ctx, root, templateDSN, env); err != nil {
			return fail(err)
		}
	}
	if err := execSQL(ctx, admin, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = 'template' AND pid <> pg_backend_pid()`); err != nil {
		return fail(err)
	}
	return &Postgres{
		adminDSN: admin,
		close: func(ctx context.Context) error {
			return ctr.Terminate(ctx)
		},
	}, nil
}

// Close terminates the Postgres container.
func (p *Postgres) Close(ctx context.Context) error {
	return p.close(ctx)
}

// Database returns a DSN for a database cloned from the migrated template.
// Creation is serialized so two tests do not copy the template at once.
func (p *Postgres) Database(ctx context.Context, t *testing.T) string {
	t.Helper()
	name := databaseName(t.Name())
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := execSQL(ctx, p.adminDSN, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", name, templateDBName)); err != nil {
		t.Fatalf("create database %s: %v", name, err)
	}
	dsn, err := withDatabase(p.adminDSN, name)
	if err != nil {
		t.Fatal(err)
	}
	return dsn
}

func databaseName(testName string) string {
	var b strings.Builder
	b.WriteString("t_")
	for _, r := range strings.ToLower(testName) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	name := b.String()
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

func execSQL(ctx context.Context, dsn, sql string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, sql)
	return err
}

func applyAtlas(ctx context.Context, root, databaseURL, env string) error {
	// The Atlas image cannot use the test process's 127.0.0.1. The published
	// port is reached through the host gateway instead.
	inside, err := urlWithHost(databaseURL, "host.docker.internal")
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"--add-host=host.docker.internal:host-gateway",
		"-e", "DATABASE_URL="+inside,
		"-v", root+":/work",
		"-w", "/work",
		atlasImage,
		"migrate", "apply", "--allow-dirty", "--env", env,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("atlas %s: %w\n%s", env, err, out)
	}
	return nil
}

func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot locate test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..")), nil
}

func forceLoopback(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	_, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", err
	}
	u.Host = net.JoinHostPort("127.0.0.1", port)
	return u.String(), nil
}

func urlWithHost(dsn, host string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	_, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", err
	}
	u.Host = net.JoinHostPort(host, port)
	return u.String(), nil
}

func withDatabase(dsn, name string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	u.Path = "/" + name
	return u.String(), nil
}
