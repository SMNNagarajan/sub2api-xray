package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBuildXrayOutboundVMess(t *testing.T) {
	node := map[string]any{
		"add":  "vmess.example.com",
		"port": "443",
		"id":   "11111111-1111-1111-1111-111111111111",
		"aid":  0,
		"scy":  "auto",
		"net":  "ws",
		"type": "none",
		"host": "cdn.example.com",
		"path": "/ws",
		"tls":  "tls",
		"sni":  "sni.example.com",
	}
	raw, _ := json.Marshal(node)
	out, err := buildXrayOutbound("vmess://"+base64.RawStdEncoding.EncodeToString(raw), &Proxy{Kind: "xray"})
	if err != nil {
		t.Fatalf("buildXrayOutbound returned error: %v", err)
	}
	if out["protocol"] != "vmess" {
		t.Fatalf("protocol mismatch: %v", out["protocol"])
	}
	stream := out["streamSettings"].(map[string]any)
	if stream["network"] != "ws" || stream["security"] != "tls" {
		t.Fatalf("stream settings mismatch: %#v", stream)
	}
}

func TestBuildXrayOutboundVLESSReality(t *testing.T) {
	out, err := buildXrayOutbound("vless://11111111-1111-1111-1111-111111111111@vless.example.com:443?security=reality&type=grpc&sni=sni.example.com&pbk=pub&sid=abc&serviceName=svc", &Proxy{Kind: "xray"})
	if err != nil {
		t.Fatalf("buildXrayOutbound returned error: %v", err)
	}
	if out["protocol"] != "vless" {
		t.Fatalf("protocol mismatch: %v", out["protocol"])
	}
	stream := out["streamSettings"].(map[string]any)
	if stream["network"] != "grpc" || stream["security"] != "reality" {
		t.Fatalf("stream settings mismatch: %#v", stream)
	}
	reality := stream["realitySettings"].(map[string]any)
	if reality["publicKey"] != "pub" || reality["shortId"] != "abc" {
		t.Fatalf("reality settings mismatch: %#v", reality)
	}
}

func TestBuildXrayOutboundShadowsocks(t *testing.T) {
	userInfo := base64.RawURLEncoding.EncodeToString([]byte("aes-128-gcm:secret"))
	out, err := buildXrayOutbound("ss://"+userInfo+"@ss.example.com:8388#node", &Proxy{Kind: "xray"})
	if err != nil {
		t.Fatalf("buildXrayOutbound returned error: %v", err)
	}
	if out["protocol"] != "shadowsocks" {
		t.Fatalf("protocol mismatch: %v", out["protocol"])
	}
}

func TestPinUserOwnedXrayOutboundRejectsPrivateEndpoint(t *testing.T) {
	outbound := map[string]any{
		"settings": map[string]any{
			"servers": []map[string]any{{"address": "127.0.0.1", "port": 443}},
		},
	}
	if err := pinUserOwnedXrayOutbound(context.Background(), outbound); err == nil {
		t.Fatal("expected private xray endpoint to be rejected")
	}
}

func TestPinUserOwnedXrayOutboundPinsPublicLiteral(t *testing.T) {
	server := map[string]any{"address": "8.8.8.8", "port": 443}
	outbound := map[string]any{
		"settings": map[string]any{"servers": []map[string]any{server}},
	}
	if err := pinUserOwnedXrayOutbound(context.Background(), outbound); err != nil {
		t.Fatalf("pin public xray endpoint: %v", err)
	}
	if server["address"] != "8.8.8.8" {
		t.Fatalf("unexpected pinned address: %v", server["address"])
	}
}

func TestBuildXrayRuntimeConfigBlocksPrivateDestinationsForUserResources(t *testing.T) {
	outbound := map[string]any{
		"tag":      "sub2api-out",
		"protocol": "socks",
		"settings": map[string]any{},
	}
	config := buildXrayRuntimeConfig(1080, outbound, true)
	routing, ok := config["routing"].(map[string]any)
	if !ok || routing["domainStrategy"] != "IPOnDemand" {
		t.Fatalf("protected runtime is missing DNS-aware routing: %#v", config["routing"])
	}
	rules, ok := routing["rules"].([]map[string]any)
	if !ok || len(rules) != 1 || rules[0]["outboundTag"] != "sub2api-block" {
		t.Fatalf("protected runtime is missing the private destination rule: %#v", routing["rules"])
	}
	cidrs, ok := rules[0]["ip"].([]string)
	if !ok || !containsString(cidrs, "10.0.0.0/8") || !containsString(cidrs, "172.16.0.0/12") || !containsString(cidrs, "192.168.0.0/16") {
		t.Fatalf("protected runtime does not block private IPv4 ranges: %#v", rules[0]["ip"])
	}
	outbounds, ok := config["outbounds"].([]map[string]any)
	if !ok || len(outbounds) != 2 || outbounds[1]["protocol"] != "blackhole" {
		t.Fatalf("protected runtime is missing the blackhole outbound: %#v", config["outbounds"])
	}

	systemConfig := buildXrayRuntimeConfig(1081, outbound, false)
	if _, ok := systemConfig["routing"]; ok {
		t.Fatalf("system runtime unexpectedly received user-only routing restrictions: %#v", systemConfig["routing"])
	}
}

func TestXrayInstanceHashIncludesOwnerScope(t *testing.T) {
	ownerA := int64(7)
	ownerB := int64(8)
	outbound := map[string]any{"protocol": "socks", "settings": map[string]any{}}
	a := xrayInstanceHash("node", &Proxy{ID: 1, Kind: "xray", OwnerUserID: &ownerA}, outbound)
	b := xrayInstanceHash("node", &Proxy{ID: 1, Kind: "xray", OwnerUserID: &ownerB}, outbound)
	system := xrayInstanceHash("node", &Proxy{ID: 1, Kind: "xray"}, outbound)
	if a == b || a == system || b == system {
		t.Fatalf("owner scopes produced the same runtime hash: a=%s b=%s system=%s", a, b, system)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestXrayRuntimeManagerStartsRealProcess(t *testing.T) {
	if !strings.EqualFold(os.Getenv("XRAY_RUNTIME_E2E"), "true") {
		t.Skip("set XRAY_RUNTIME_E2E=true and XRAY_BIN to run the real xray runtime test")
	}
	bin := strings.TrimSpace(os.Getenv("XRAY_BIN"))
	if bin == "" {
		t.Skip("XRAY_BIN is required for the real xray runtime test")
	}

	manager := NewXrayRuntimeManager(bin, t.TempDir())
	defer func() { _ = manager.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	proxy := &Proxy{
		ID:   time.Now().UnixNano(),
		Kind: "xray",
		Extra: map[string]any{
			"outbound": map[string]any{
				"tag":      "direct",
				"protocol": "freedom",
				"settings": map[string]any{},
			},
		},
	}
	proxyURL, err := manager.ProxyURL(ctx, proxy)
	if err != nil {
		t.Fatalf("ProxyURL returned error: %v", err)
	}
	if !strings.HasPrefix(proxyURL, "socks5h://127.0.0.1:") {
		t.Fatalf("unexpected local SOCKS URL: %s", proxyURL)
	}

	hostPort := strings.TrimPrefix(proxyURL, "socks5h://")
	conn, err := net.DialTimeout("tcp", hostPort, time.Second)
	if err != nil {
		t.Fatalf("xray local SOCKS port is not reachable: %v", err)
	}
	_ = conn.Close()

	secondURL, err := manager.ProxyURL(ctx, proxy)
	if err != nil {
		t.Fatalf("second ProxyURL returned error: %v", err)
	}
	if secondURL != proxyURL {
		t.Fatalf("xray runtime did not reuse the live instance: first=%s second=%s", proxyURL, secondURL)
	}
}

func TestXrayRuntimeManagerConcurrentStartAndClose(t *testing.T) {
	workDir := t.TempDir()
	manager := NewXrayRuntimeManager("xray-test-helper", workDir)
	var starts atomic.Int32
	manager.commandFactory = func(_, configPath string) *exec.Cmd {
		starts.Add(1)
		cmd := exec.Command(os.Args[0], "-test.run=^TestXrayRuntimeHelperProcess$")
		cmd.Env = append(os.Environ(),
			"SUB2API_XRAY_HELPER=1",
			"SUB2API_XRAY_HELPER_CONFIG="+configPath,
		)
		return cmd
	}

	proxy := &Proxy{
		ID:   91234,
		Kind: "xray",
		Extra: map[string]any{
			"outbound": map[string]any{
				"tag":      "direct",
				"protocol": "freedom",
				"settings": map[string]any{},
			},
		},
	}

	const callers = 12
	start := make(chan struct{})
	urls := make(chan string, callers)
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got, err := manager.ProxyURL(ctx, proxy)
			if err != nil {
				errs <- err
				return
			}
			urls <- got
		}()
	}
	close(start)
	wg.Wait()
	close(urls)
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent ProxyURL returned error: %v", err)
	}
	var first string
	for got := range urls {
		if first == "" {
			first = got
		}
		if got != first {
			t.Fatalf("concurrent calls returned different runtimes: first=%s got=%s", first, got)
		}
	}
	if got := starts.Load(); got != 1 {
		t.Fatalf("expected exactly one xray process, got %d", got)
	}

	if err := manager.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	entries, err := os.ReadDir(workDir)
	if err != nil {
		t.Fatalf("read work dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("xray secret files were not removed: %#v", entries)
	}
	if _, err := manager.ProxyURL(context.Background(), proxy); err == nil {
		t.Fatal("closed manager unexpectedly accepted a new runtime")
	}
}

func TestXrayRuntimeManagerEnforcesInstanceLimit(t *testing.T) {
	manager := NewXrayRuntimeManager("xray-test-helper", t.TempDir())
	manager.maxInstances = 1
	manager.commandFactory = func(_, configPath string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=^TestXrayRuntimeHelperProcess$")
		cmd.Env = append(os.Environ(),
			"SUB2API_XRAY_HELPER=1",
			"SUB2API_XRAY_HELPER_CONFIG="+configPath,
		)
		return cmd
	}
	defer func() { _ = manager.Close() }()

	proxy := func(id int64) *Proxy {
		return &Proxy{
			ID:   id,
			Kind: "xray",
			Extra: map[string]any{
				"outbound": map[string]any{
					"tag":      "direct",
					"protocol": "freedom",
					"settings": map[string]any{},
				},
			},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := manager.ProxyURL(ctx, proxy(1)); err != nil {
		t.Fatalf("start first runtime: %v", err)
	}
	if _, err := manager.ProxyURL(ctx, proxy(2)); err == nil || !strings.Contains(err.Error(), "instance limit") {
		t.Fatalf("expected instance limit error, got %v", err)
	}
	if err := manager.Stop(1); err != nil {
		t.Fatalf("stop first runtime: %v", err)
	}
	if _, err := manager.ProxyURL(ctx, proxy(2)); err != nil {
		t.Fatalf("start runtime after releasing capacity: %v", err)
	}
}

func TestXrayRuntimeManagerEnforcesPerUserInstanceLimit(t *testing.T) {
	manager := NewXrayRuntimeManager("xray-test-helper", t.TempDir())
	manager.maxInstances = 4
	manager.maxInstancesPerUser = 1
	manager.commandFactory = func(_, configPath string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=^TestXrayRuntimeHelperProcess$")
		cmd.Env = append(os.Environ(),
			"SUB2API_XRAY_HELPER=1",
			"SUB2API_XRAY_HELPER_CONFIG="+configPath,
		)
		return cmd
	}
	defer func() { _ = manager.Close() }()

	proxy := func(id, ownerID int64) *Proxy {
		return &Proxy{
			ID: id, Kind: "xray", OwnerUserID: &ownerID,
			Extra: map[string]any{"outbound": map[string]any{
				"protocol": "socks",
				"settings": map[string]any{"servers": []map[string]any{{"address": "8.8.8.8", "port": 1080}}},
			}},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := manager.ProxyURL(ctx, proxy(1, 7)); err != nil {
		t.Fatalf("start first user runtime: %v", err)
	}
	if _, err := manager.ProxyURL(ctx, proxy(2, 7)); err == nil || !strings.Contains(err.Error(), "per-user instance limit") {
		t.Fatalf("expected per-user instance limit error, got %v", err)
	}
	if _, err := manager.ProxyURL(ctx, proxy(3, 8)); err != nil {
		t.Fatalf("another owner should retain independent capacity: %v", err)
	}
}

func TestXrayRuntimeHelperProcess(t *testing.T) {
	if os.Getenv("SUB2API_XRAY_HELPER") != "1" {
		return
	}
	raw, err := os.ReadFile(os.Getenv("SUB2API_XRAY_HELPER_CONFIG"))
	if err != nil {
		t.Fatalf("read helper config: %v", err)
	}
	var cfg struct {
		Inbounds []struct {
			Port int `json:"port"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil || len(cfg.Inbounds) == 0 {
		t.Fatalf("decode helper config: %v", err)
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Inbounds[0].Port)))
	if err != nil {
		t.Fatalf("listen helper port: %v", err)
	}
	defer func() { _ = listener.Close() }()
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		_ = conn.Close()
	}
}
