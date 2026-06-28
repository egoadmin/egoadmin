//go:build e2e

package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type environment struct {
	runID      string
	repoRoot   string
	root       string
	project    string
	ports      ports
	httpClient *http.Client
	processes  []*serviceProcess
	keepTmp    bool
}

type ports struct {
	MySQLIDGen     int
	MySQLGateway   int
	MySQLUser      int
	RedisGateway   int
	RedisUser      int
	EtcdClient     int
	EtcdPeer       int
	MinIOAPI       int
	MinIOConsole   int
	ImageProcessor int
	JaegerOTLP     int
	JaegerHTTP     int
	GatewayHTTP    int
	GatewayGRPC    int
	GatewayGovern  int
	IDGenHTTP      int
	IDGenGRPC      int
	IDGenGovern    int
	UserHTTP       int
	UserGRPC       int
	UserGovern     int
}

type serviceProcess struct {
	name string
	cmd  *exec.Cmd
	log  *os.File
}

func setupEnvironment(ctx context.Context) (*environment, error) {
	runID := os.Getenv("E2E_RUN_ID")
	if runID == "" {
		runID = fmt.Sprintf("egoadmin-e2e-%d", time.Now().UnixNano())
	}
	runID = sanitizeProjectName(runID)

	repoRoot, err := findRepoRoot()
	if err != nil {
		return nil, err
	}
	p, err := allocatePorts(21)
	if err != nil {
		return nil, err
	}
	e := &environment{
		runID:    runID,
		repoRoot: repoRoot,
		root:     filepath.Join(repoRoot, "test", "e2e", ".tmp", runID),
		project:  runID,
		ports:    p,
		keepTmp:  os.Getenv("E2E_KEEP_TMP") == "1",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	if err := os.MkdirAll(filepath.Join(e.root, "logs"), 0o755); err != nil {
		return nil, err
	}
	if err := e.writeComposeFile(); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, err
	}
	if err := e.writeConfigFiles(); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, err
	}
	if err := e.compose(ctx, "up", "-d", "--wait"); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, err
	}
	if err := e.waitDatabases(ctx); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, err
	}
	if err := e.applyAtlas(ctx, "idgen", e.idgenAtlasURL()); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, err
	}
	if err := e.applyAtlas(ctx, "user", e.userAtlasURL()); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, err
	}
	if err := e.applyAtlas(ctx, "gateway", e.gatewayAtlasURL()); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, err
	}
	for _, item := range []struct {
		port int
		db   string
	}{
		{e.ports.MySQLIDGen, "egoadmin_idgen"},
		{e.ports.MySQLUser, "egoadmin_user"},
		{e.ports.MySQLGateway, "egoadmin_gateway"},
	} {
		if err := e.verifyAtlasRevision(ctx, item.port, item.db); err != nil {
			_ = e.Cleanup(context.Background())
			return nil, err
		}
	}
	if err := e.verifyIDGenTablesOwnedByIDGen(ctx); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, err
	}
	if err := e.startService(ctx, "idgen", "./cmd/idgen", filepath.Join(e.root, "configs", "idgen.toml"), "egoadmin-idgen"); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, err
	}
	if err := e.waitGRPCHealth(ctx, e.ports.IDGenGRPC); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, fmt.Errorf("wait idgen grpc health: %w", err)
	}
	if err := e.waitHTTPPort(ctx, e.ports.IDGenHTTP, "/readyz", http.StatusOK); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, fmt.Errorf("wait idgen readyz: %w", err)
	}
	if err := e.startService(ctx, "user", "./cmd/user", filepath.Join(e.root, "configs", "user.toml"), "egoadmin-user"); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, err
	}
	if err := e.waitGRPCHealth(ctx, e.ports.UserGRPC); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, fmt.Errorf("wait user grpc health: %w", err)
	}
	if err := e.waitHTTPPort(ctx, e.ports.UserHTTP, "/readyz", http.StatusOK); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, fmt.Errorf("wait user readyz: %w", err)
	}
	if err := e.startService(ctx, "gateway", "./cmd/gateway", filepath.Join(e.root, "configs", "gateway.toml"), "egoadmin-gateway"); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, err
	}
	if err := e.waitGRPCHealth(ctx, e.ports.GatewayGRPC); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, fmt.Errorf("wait gateway grpc health: %w", err)
	}
	if err := e.waitHTTP(ctx, "/readyz", http.StatusOK); err != nil {
		_ = e.Cleanup(context.Background())
		return nil, fmt.Errorf("wait gateway readyz: %w", err)
	}
	return e, nil
}

func (e *environment) Cleanup(ctx context.Context) error {
	var errs []error
	for i := len(e.processes) - 1; i >= 0; i-- {
		if err := e.stopProcess(ctx, e.processes[i]); err != nil {
			errs = append(errs, err)
		}
	}
	if err := e.compose(ctx, "down", "-v", "--remove-orphans"); err != nil {
		errs = append(errs, err)
	}
	if !e.keepTmp {
		if err := os.RemoveAll(e.root); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (e *environment) DumpDiagnostics() {
	if e == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "\n--- e2e diagnostics: %s ---\n", e.runID)
	e.dumpComposeStatus(context.Background())
	e.dumpComposeLogTail(context.Background(), "image-processor", 160)
	for _, name := range []string{"idgen", "user", "gateway"} {
		e.dumpLogTail(name, 160)
	}
	fmt.Fprintln(os.Stderr, "--- end e2e diagnostics ---")
}

func (e *environment) compose(ctx context.Context, args ...string) error {
	allArgs := append([]string{"compose", "-p", e.project, "-f", filepath.Join(e.root, "docker-compose.yml")}, args...)
	cmd := exec.CommandContext(ctx, "docker", allArgs...)
	cmd.Dir = e.repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (e *environment) startService(ctx context.Context, name, goCmd, configPath, egoName string) error {
	logPath := filepath.Join(e.root, "logs", name+".log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "go", "run", "-race", goCmd, "--config="+configPath)
	cmd.Dir = e.repoRoot
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(),
		"EGO_NAME="+egoName,
		"EGO_DEBUG=false",
		"EGOADMIN_ATLAS_MIGRATED=true",
	)
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("start %s: %w", name, err)
	}
	e.processes = append(e.processes, &serviceProcess{name: name, cmd: cmd, log: logFile})
	return nil
}

func (e *environment) stopProcess(ctx context.Context, proc *serviceProcess) error {
	if proc == nil || proc.cmd == nil || proc.cmd.Process == nil {
		return nil
	}
	done := make(chan error, 1)
	go func() {
		done <- proc.cmd.Wait()
	}()
	_ = proc.cmd.Process.Signal(os.Interrupt)
	select {
	case err := <-done:
		_ = proc.log.Close()
		if err == nil {
			return nil
		}
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == -1 {
			return nil
		}
		return nil
	case <-time.After(5 * time.Second):
		_ = proc.cmd.Process.Kill()
		err := <-done
		_ = proc.log.Close()
		if err != nil {
			return nil
		}
		return nil
	case <-ctx.Done():
		_ = proc.cmd.Process.Kill()
		_ = proc.log.Close()
		return ctx.Err()
	}
}

func (e *environment) waitDatabases(ctx context.Context) error {
	if err := e.waitMySQL(ctx, e.ports.MySQLIDGen, "egoadmin_idgen"); err != nil {
		return err
	}
	if err := e.waitMySQL(ctx, e.ports.MySQLGateway, "egoadmin_gateway"); err != nil {
		return err
	}
	if err := e.waitMySQL(ctx, e.ports.MySQLUser, "egoadmin_user"); err != nil {
		return err
	}
	return nil
}

func (e *environment) verifyIDGenTablesOwnedByIDGen(ctx context.Context) error {
	for _, item := range []struct {
		port    int
		db      string
		wantSeg bool
		wantMac bool
	}{
		{e.ports.MySQLIDGen, "egoadmin_idgen", true, true},
		{e.ports.MySQLUser, "egoadmin_user", false, false},
		{e.ports.MySQLGateway, "egoadmin_gateway", false, false},
	} {
		hasSegment, err := e.tableExists(ctx, item.port, item.db, "idgen_segment")
		if err != nil {
			return err
		}
		if hasSegment != item.wantSeg {
			return fmt.Errorf("%s idgen_segment exists=%v, want %v", item.db, hasSegment, item.wantSeg)
		}
		hasLease, err := e.tableExists(ctx, item.port, item.db, "idgen_machine_lease")
		if err != nil {
			return err
		}
		if hasLease != item.wantMac {
			return fmt.Errorf("%s idgen_machine_lease exists=%v, want %v", item.db, hasLease, item.wantMac)
		}
	}
	return nil
}

func (e *environment) tableExists(ctx context.Context, port int, dbName string, tableName string) (bool, error) {
	dsn := fmt.Sprintf("egoadmin:egoadmin@tcp(127.0.0.1:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", port, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return false, err
	}
	defer db.Close()

	var count int
	err = db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.tables
WHERE table_schema = ? AND table_name = ?
`, dbName, tableName).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("query table %s.%s: %w", dbName, tableName, err)
	}
	return count > 0, nil
}

func (e *environment) waitMySQL(ctx context.Context, port int, dbName string) error {
	dsn := fmt.Sprintf("egoadmin:egoadmin@tcp(127.0.0.1:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", port, dbName)
	deadline := time.Now().Add(90 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		db, err := sql.Open("mysql", dsn)
		if err == nil {
			err = db.PingContext(ctx)
			_ = db.Close()
		}
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(time.Second)
	}
	return fmt.Errorf("mysql %s on port %d not ready: %w", dbName, port, lastErr)
}

func (e *environment) verifyAtlasRevision(ctx context.Context, port int, dbName string) error {
	dsn := fmt.Sprintf("egoadmin:egoadmin@tcp(127.0.0.1:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", port, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	var count int
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM atlas_schema_revisions").Scan(&count); err != nil {
		return fmt.Errorf("query atlas_schema_revisions in %s: %w", dbName, err)
	}
	if count == 0 {
		return fmt.Errorf("atlas_schema_revisions in %s is empty", dbName)
	}
	return nil
}

func (e *environment) openUserDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := fmt.Sprintf("egoadmin:egoadmin@tcp(127.0.0.1:%d)/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local", e.ports.MySQLUser)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open user db: %v", err)
	}
	return db
}

func (e *environment) applyAtlas(ctx context.Context, service, atlasURL string) error {
	cmd := exec.CommandContext(ctx, "atlas", "migrate", "apply", "--env", "local", "--var", "service="+service, "--config", "file://atlas/atlas.hcl")
	cmd.Dir = e.repoRoot
	cmd.Env = append(os.Environ(), "ATLAS_URL="+atlasURL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("atlas apply %s: %w\n%s", service, err, string(out))
	}
	return nil
}

func (e *environment) dumpComposeStatus(ctx context.Context) {
	allArgs := []string{
		"compose", "-p", e.project,
		"-f", filepath.Join(e.root, "docker-compose.yml"),
		"ps", "-a",
	}
	cmd := exec.CommandContext(ctx, "docker", allArgs...)
	cmd.Dir = e.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "docker compose ps failed: %v\n%s\n", err, string(out))
		return
	}
	fmt.Fprintf(os.Stderr, "\n[compose ps]\n%s\n", string(out))
}

func (e *environment) dumpComposeLogTail(ctx context.Context, service string, limit int) {
	allArgs := []string{
		"compose", "-p", e.project,
		"-f", filepath.Join(e.root, "docker-compose.yml"),
		"logs", "--tail", fmt.Sprintf("%d", limit), service,
	}
	cmd := exec.CommandContext(ctx, "docker", allArgs...)
	cmd.Dir = e.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n[compose logs %s failed: %v]\n%s\n", service, err, string(out))
		return
	}
	fmt.Fprintf(os.Stderr, "\n[compose logs %s]\n%s\n", service, string(out))
}

func (e *environment) dumpLogTail(name string, limit int) {
	path := filepath.Join(e.root, "logs", name+".log")
	file, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n[%s log unavailable: %v]\n", name, err)
		return
	}
	defer file.Close()

	lines := make([]string, 0, limit)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > limit {
			copy(lines, lines[1:])
			lines = lines[:limit]
		}
	}
	if err = scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "\n[%s log scan failed: %v]\n", name, err)
		return
	}
	fmt.Fprintf(os.Stderr, "\n[%s log tail]\n", name)
	for _, line := range lines {
		fmt.Fprintln(os.Stderr, line)
	}
}

func (e *environment) waitGRPCHealth(ctx context.Context, port int) error {
	deadline := time.Now().Add(90 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		dialCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		conn, err := grpc.DialContext(dialCtx, fmt.Sprintf("127.0.0.1:%d", port), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
		cancel()
		if err == nil {
			healthCtx, healthCancel := context.WithTimeout(ctx, 2*time.Second)
			resp, checkErr := healthpb.NewHealthClient(conn).Check(healthCtx, &healthpb.HealthCheckRequest{})
			healthCancel()
			_ = conn.Close()
			if checkErr == nil && resp.GetStatus() == healthpb.HealthCheckResponse_SERVING {
				return nil
			}
			if checkErr != nil {
				lastErr = checkErr
			} else {
				lastErr = fmt.Errorf("status %s", resp.GetStatus().String())
			}
		} else {
			lastErr = err
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("grpc health port %d not serving: %w", port, lastErr)
}

func (e *environment) waitHTTP(ctx context.Context, path string, status int) error {
	return e.waitHTTPPort(ctx, e.ports.GatewayHTTP, path, status)
}

func (e *environment) waitHTTPPort(ctx context.Context, port int, path string, status int) error {
	deadline := time.Now().Add(90 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, httpURLForPort(port, path), nil)
		if err != nil {
			return err
		}
		resp, err := e.httpClient.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == status {
				return nil
			}
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(time.Second)
	}
	return lastErr
}

func (e *environment) idgenAtlasURL() string {
	return fmt.Sprintf("mysql://egoadmin:egoadmin@127.0.0.1:%d/egoadmin_idgen?charset=utf8mb4&parseTime=True&loc=Local", e.ports.MySQLIDGen)
}

func (e *environment) gatewayAtlasURL() string {
	return fmt.Sprintf("mysql://egoadmin:egoadmin@127.0.0.1:%d/egoadmin_gateway?charset=utf8mb4&parseTime=True&loc=Local", e.ports.MySQLGateway)
}

func (e *environment) userAtlasURL() string {
	return fmt.Sprintf("mysql://egoadmin:egoadmin@127.0.0.1:%d/egoadmin_user?charset=utf8mb4&parseTime=True&loc=Local", e.ports.MySQLUser)
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd, nil
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return "", fmt.Errorf("go.mod not found from %s", wd)
		}
		wd = parent
	}
}

func allocatePorts(n int) (ports, error) {
	listeners := make([]net.Listener, 0, n)
	defer func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
	}()
	values := make([]int, 0, n)
	for i := 0; i < n; i++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return ports{}, err
		}
		listeners = append(listeners, listener)
		values = append(values, listener.Addr().(*net.TCPAddr).Port)
	}
	return ports{
		MySQLIDGen:     values[0],
		MySQLGateway:   values[1],
		MySQLUser:      values[2],
		RedisGateway:   values[3],
		RedisUser:      values[4],
		EtcdClient:     values[5],
		EtcdPeer:       values[6],
		MinIOAPI:       values[7],
		MinIOConsole:   values[8],
		ImageProcessor: values[9],
		JaegerOTLP:     values[10],
		JaegerHTTP:     values[11],
		GatewayHTTP:    values[12],
		GatewayGRPC:    values[13],
		GatewayGovern:  values[14],
		IDGenHTTP:      values[15],
		IDGenGRPC:      values[16],
		IDGenGovern:    values[17],
		UserHTTP:       values[18],
		UserGRPC:       values[19],
		UserGovern:     values[20],
	}, nil
}

func sanitizeProjectName(in string) string {
	out := strings.ToLower(in)
	replacer := strings.NewReplacer("_", "-", ".", "-")
	out = replacer.Replace(out)
	var b strings.Builder
	for _, r := range out {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	out = strings.Trim(b.String(), "-")
	if out == "" {
		return "egoadmin-e2e"
	}
	return out
}
