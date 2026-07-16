package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	xrayProxyUnavailableURL      = "xray://unavailable"
	defaultXrayMaxInstances      = 64
	defaultXrayMaxInstancesUser  = 16
	maximumXrayConfiguredRuntime = 1024
)

var xrayBlockedDestinationCIDRs = []string{
	"0.0.0.0/8",
	"10.0.0.0/8",
	"100.64.0.0/10",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"224.0.0.0/4",
	"240.0.0.0/4",
	"::/128",
	"::1/128",
	"fc00::/7",
	"fe80::/10",
	"ff00::/8",
}

var (
	defaultXrayRuntimeOnce sync.Once
	defaultXrayRuntime     *XrayRuntimeManager
)

// DefaultXrayRuntimeManager returns the process manager used by Proxy.URL for kind=xray proxies.
func DefaultXrayRuntimeManager() *XrayRuntimeManager {
	defaultXrayRuntimeOnce.Do(func() {
		defaultXrayRuntime = NewXrayRuntimeManager(os.Getenv("XRAY_BIN"), os.Getenv("XRAY_WORK_DIR"))
	})
	return defaultXrayRuntime
}

type XrayRuntimeManager struct {
	mu                  sync.Mutex
	bin                 string
	workDir             string
	instances           map[int64]*xrayRuntimeInstance
	maxInstances        int
	maxInstancesPerUser int
	closed              bool
	commandFactory      func(bin, configPath string) *exec.Cmd
}

type xrayRuntimeInstance struct {
	proxyID     int64
	ownerUserID *int64
	hash        string
	port        int
	cmd         *exec.Cmd
	configPath  string
	logPath     string
	done        chan error
}

func NewXrayRuntimeManager(bin, workDir string) *XrayRuntimeManager {
	if strings.TrimSpace(workDir) == "" {
		workDir = filepath.Join(os.TempDir(), "sub2api-xray")
	}
	maxInstances := parseXrayMaxInstances(os.Getenv("XRAY_MAX_INSTANCES"))
	maxInstancesPerUser := parseXrayMaxInstancesPerUser(os.Getenv("XRAY_MAX_INSTANCES_PER_USER"))
	if maxInstancesPerUser > maxInstances {
		maxInstancesPerUser = maxInstances
	}
	return &XrayRuntimeManager{
		bin:                 strings.TrimSpace(bin),
		workDir:             workDir,
		instances:           map[int64]*xrayRuntimeInstance{},
		maxInstances:        maxInstances,
		maxInstancesPerUser: maxInstancesPerUser,
	}
}

func parseXrayMaxInstances(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return defaultXrayMaxInstances
	}
	if limit > maximumXrayConfiguredRuntime {
		return maximumXrayConfiguredRuntime
	}
	return limit
}

func parseXrayMaxInstancesPerUser(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return defaultXrayMaxInstancesUser
	}
	if limit > maximumXrayConfiguredRuntime {
		return maximumXrayConfiguredRuntime
	}
	return limit
}

func (m *XrayRuntimeManager) ProxyURL(ctx context.Context, p *Proxy) (string, error) {
	if p == nil {
		return "", errors.New("proxy is nil")
	}
	if !strings.EqualFold(p.Kind, "xray") {
		return p.StandardURL(), nil
	}
	raw := xrayRawNode(p)
	outbound, err := buildXrayOutbound(raw, p)
	if err != nil {
		return "", err
	}
	if p.OwnerUserID != nil {
		if err := pinUserOwnedXrayOutbound(ctx, outbound); err != nil {
			return "", fmt.Errorf("xray outbound endpoint is not public: %w", err)
		}
		outbound["tag"] = "sub2api-out"
	}
	fingerprint := xrayInstanceHash(raw, p, outbound)

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return "", errors.New("xray runtime manager is closed")
	}
	if inst := m.instances[p.ID]; inst != nil && inst.hash == fingerprint && inst.alive() {
		return localSocksURL(inst.port), nil
	}
	if old := m.instances[p.ID]; old != nil {
		delete(m.instances, p.ID)
		_ = old.stop()
	}
	for id, inst := range m.instances {
		if inst != nil && inst.alive() {
			continue
		}
		delete(m.instances, id)
		_ = inst.stop()
	}
	if len(m.instances) >= m.maxInstances {
		return "", fmt.Errorf("xray runtime instance limit reached (%d)", m.maxInstances)
	}
	if p.OwnerUserID != nil {
		ownerInstances := 0
		for _, inst := range m.instances {
			if inst != nil && inst.ownerUserID != nil && *inst.ownerUserID == *p.OwnerUserID && inst.alive() {
				ownerInstances++
			}
		}
		if ownerInstances >= m.maxInstancesPerUser {
			return "", fmt.Errorf("xray runtime per-user instance limit reached (%d)", m.maxInstancesPerUser)
		}
	}

	inst, err := m.start(ctx, p.ID, p.OwnerUserID, fingerprint, outbound, p.OwnerUserID != nil)
	if err != nil {
		return "", err
	}

	m.instances[p.ID] = inst
	return localSocksURL(inst.port), nil
}

func pinUserOwnedXrayOutbound(ctx context.Context, outbound map[string]any) error {
	settings, _ := outbound["settings"].(map[string]any)
	if settings == nil {
		return errors.New("xray outbound settings are missing")
	}
	pinned := 0
	for _, key := range []string{"vnext", "servers"} {
		servers, ok := settings[key].([]map[string]any)
		if !ok {
			if rawServers, rawOK := settings[key].([]any); rawOK {
				servers = make([]map[string]any, 0, len(rawServers))
				for _, rawServer := range rawServers {
					if server, mapOK := rawServer.(map[string]any); mapOK {
						servers = append(servers, server)
					}
				}
			}
		}
		for _, server := range servers {
			host := strings.TrimSpace(stringFromMap(server, "address"))
			if host == "" {
				continue
			}
			ips, err := resolveExternalHostIPs(ctx, host)
			if err != nil {
				return err
			}
			if len(ips) == 0 {
				return fmt.Errorf("xray endpoint %q resolved to no addresses", host)
			}
			preserveXrayTLSServerName(outbound, host)
			server["address"] = ips[0].String()
			pinned++
		}
	}
	if pinned == 0 {
		return errors.New("xray outbound has no server address")
	}
	return nil
}

func preserveXrayTLSServerName(outbound map[string]any, host string) {
	if net.ParseIP(host) != nil {
		return
	}
	stream, _ := outbound["streamSettings"].(map[string]any)
	if stream == nil {
		return
	}
	for _, key := range []string{"tlsSettings", "realitySettings"} {
		settings, _ := stream[key].(map[string]any)
		if settings == nil || strings.TrimSpace(stringFromMap(settings, "serverName")) != "" {
			continue
		}
		settings["serverName"] = host
	}
}

// Stop terminates one proxy runtime and removes its on-disk secrets.
func (m *XrayRuntimeManager) Stop(proxyID int64) error {
	if m == nil || proxyID <= 0 {
		return nil
	}
	m.mu.Lock()
	inst := m.instances[proxyID]
	delete(m.instances, proxyID)
	m.mu.Unlock()
	if inst == nil {
		return nil
	}
	return inst.stop()
}

// Close terminates every managed runtime. A closed manager cannot be reused.
func (m *XrayRuntimeManager) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	instances := make([]*xrayRuntimeInstance, 0, len(m.instances))
	for id, inst := range m.instances {
		instances = append(instances, inst)
		delete(m.instances, id)
	}
	m.mu.Unlock()

	var errs []error
	for _, inst := range instances {
		if err := inst.stop(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m *XrayRuntimeManager) start(ctx context.Context, proxyID int64, ownerUserID *int64, hash string, outbound map[string]any, blockPrivateDestinations bool) (*xrayRuntimeInstance, error) {
	bin, err := m.resolveBinary()
	if err != nil {
		return nil, err
	}
	port, err := reserveLocalPort()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(m.workDir, 0o700); err != nil {
		return nil, err
	}
	config := buildXrayRuntimeConfig(port, outbound, blockPrivateDestinations)
	rawConfig, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, err
	}
	prefix := fmt.Sprintf("proxy-%d-%s", proxyID, hash[:12])
	configPath := filepath.Join(m.workDir, prefix+".json")
	logPath := filepath.Join(m.workDir, prefix+".log")
	if err := os.WriteFile(configPath, rawConfig, 0o600); err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		_ = os.Remove(configPath)
		return nil, err
	}
	cmd := exec.CommandContext(context.Background(), bin, "run", "-config", configPath)
	if m.commandFactory != nil {
		cmd = m.commandFactory(bin, configPath)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		_ = os.Remove(configPath)
		_ = os.Remove(logPath)
		return nil, err
	}
	inst := &xrayRuntimeInstance{
		proxyID:     proxyID,
		ownerUserID: cloneInt64Pointer(ownerUserID),
		hash:        hash,
		port:        port,
		cmd:         cmd,
		configPath:  configPath,
		logPath:     logPath,
		done:        make(chan error, 1),
	}
	go func() {
		inst.done <- cmd.Wait()
		close(inst.done)
		_ = logFile.Close()
		_ = os.Remove(configPath)
		_ = os.Remove(logPath)
	}()

	waitCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := waitForLocalPort(waitCtx, port, inst.done); err != nil {
		_ = inst.stop()
		return nil, fmt.Errorf("xray runtime did not become ready: %w", err)
	}
	return inst, nil
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func buildXrayRuntimeConfig(port int, outbound map[string]any, blockPrivateDestinations bool) map[string]any {
	config := map[string]any{
		"log": map[string]any{
			"loglevel": "warning",
		},
		"inbounds": []map[string]any{
			{
				"tag":      "sub2api-in",
				"listen":   "127.0.0.1",
				"port":     port,
				"protocol": "socks",
				"settings": map[string]any{
					"auth": "noauth",
					"udp":  true,
				},
			},
		},
		"outbounds": []map[string]any{outbound},
	}
	if blockPrivateDestinations {
		config["outbounds"] = []map[string]any{
			outbound,
			{
				"tag":      "sub2api-block",
				"protocol": "blackhole",
				"settings": map[string]any{},
			},
		}
		config["routing"] = map[string]any{
			"domainStrategy": "IPOnDemand",
			"rules": []map[string]any{
				{
					"type":        "field",
					"ip":          append([]string(nil), xrayBlockedDestinationCIDRs...),
					"outboundTag": "sub2api-block",
				},
			},
		}
	}
	return config
}

func (m *XrayRuntimeManager) resolveBinary() (string, error) {
	if m.bin != "" {
		return m.bin, nil
	}
	if path, err := exec.LookPath("xray"); err == nil {
		m.bin = path
		return path, nil
	}
	return "", errors.New("xray binary not found; set XRAY_BIN")
}

func (i *xrayRuntimeInstance) alive() bool {
	if i == nil || i.cmd == nil || i.cmd.Process == nil {
		return false
	}
	select {
	case <-i.done:
		return false
	default:
		return true
	}
}

func (i *xrayRuntimeInstance) stop() error {
	if i == nil {
		return nil
	}
	if i.cmd != nil && i.cmd.Process != nil && i.alive() {
		if err := i.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		select {
		case <-i.done:
		case <-time.After(2 * time.Second):
		}
	}
	var errs []error
	for _, path := range []string{i.configPath, i.logPath} {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func reserveLocalPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitForLocalPort(ctx context.Context, port int, done <-chan error) error {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		select {
		case err := <-done:
			if err == nil {
				err = errors.New("xray exited")
			}
			return err
		case <-ctx.Done():
			if lastErr != nil {
				return lastErr
			}
			return ctx.Err()
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				return nil
			}
			lastErr = err
		}
	}
}

func localSocksURL(port int) string {
	return "socks5h://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}

func xrayRawNode(p *Proxy) string {
	if p == nil || p.Extra == nil {
		return ""
	}
	for _, key := range []string{"raw", "uri", "node", "node_uri", "share_link"} {
		if v, ok := p.Extra[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func xrayInstanceHash(raw string, p *Proxy, outbound map[string]any) string {
	h := sha256.New()
	_, _ = h.Write([]byte(raw))
	_, _ = h.Write([]byte(p.Protocol))
	_, _ = h.Write([]byte(p.Host))
	_, _ = h.Write([]byte(strconv.Itoa(p.Port)))
	if p.OwnerUserID != nil {
		_, _ = h.Write([]byte("owner:" + strconv.FormatInt(*p.OwnerUserID, 10)))
	} else {
		_, _ = h.Write([]byte("owner:system"))
	}
	if encoded, err := json.Marshal(outbound); err == nil {
		_, _ = h.Write(encoded)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func buildXrayOutbound(raw string, p *Proxy) (map[string]any, error) {
	if p != nil && p.Extra != nil {
		if outbound, ok := p.Extra["outbound"].(map[string]any); ok && len(outbound) > 0 {
			return outbound, nil
		}
		if outbound, ok := p.Extra["xray_outbound"].(map[string]any); ok && len(outbound) > 0 {
			return outbound, nil
		}
	}
	raw = strings.TrimSpace(raw)
	if raw == "" && p != nil {
		return buildXrayStandardOutbound(p)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return nil, fmt.Errorf("invalid xray node uri")
	}
	switch strings.ToLower(u.Scheme) {
	case "vmess":
		return buildVMessOutbound(raw)
	case "vless":
		return buildVLESSOutbound(u)
	case "trojan":
		return buildTrojanOutbound(u)
	case "ss", "shadowsocks":
		return buildShadowsocksOutbound(raw)
	case "socks", "socks5", "socks5h", "http", "https":
		return buildStandardURLOutbound(u)
	default:
		return nil, fmt.Errorf("unsupported xray node protocol %q", u.Scheme)
	}
}

func buildXrayStandardOutbound(p *Proxy) (map[string]any, error) {
	if p == nil || p.Host == "" || p.Port <= 0 {
		return nil, errors.New("missing xray proxy endpoint")
	}
	u := &url.URL{Scheme: p.Protocol, Host: net.JoinHostPort(p.Host, strconv.Itoa(p.Port))}
	if p.Username != "" || p.Password != "" {
		u.User = url.UserPassword(p.Username, p.Password)
	}
	return buildStandardURLOutbound(u)
}

func buildStandardURLOutbound(u *url.URL) (map[string]any, error) {
	protocol := strings.ToLower(u.Scheme)
	if protocol == "socks5" || protocol == "socks5h" {
		protocol = "socks"
	}
	if protocol == "https" {
		protocol = "http"
	}
	port := portFromURL(u)
	if u.Hostname() == "" || port <= 0 {
		return nil, errors.New("missing proxy host or port")
	}
	server := map[string]any{"address": u.Hostname(), "port": port}
	if u.User != nil {
		user := u.User.Username()
		pass, _ := u.User.Password()
		if user != "" || pass != "" {
			server["users"] = []map[string]any{{"user": user, "pass": pass}}
		}
	}
	return taggedOutbound(protocol, map[string]any{"servers": []map[string]any{server}}, nil), nil
}

func buildVMessOutbound(raw string) (map[string]any, error) {
	payload := strings.TrimSpace(raw)
	if len(payload) >= len("vmess://") && strings.EqualFold(payload[:len("vmess://")], "vmess://") {
		payload = payload[len("vmess://"):]
	}
	decoded, ok := decodeShareBase64(payload)
	if !ok {
		return nil, errors.New("invalid vmess payload")
	}
	var node map[string]any
	if err := json.Unmarshal([]byte(decoded), &node); err != nil {
		return nil, err
	}
	address := stringFromMap(node, "add")
	port := intFromAnyValue(node["port"])
	id := stringFromMap(node, "id")
	if address == "" || port <= 0 || id == "" {
		return nil, errors.New("vmess node missing address, port or id")
	}
	user := map[string]any{"id": id, "alterId": intFromAnyValue(node["aid"])}
	if security := stringFromMap(node, "scy"); security != "" {
		user["security"] = security
	} else {
		user["security"] = "auto"
	}
	settings := map[string]any{
		"vnext": []map[string]any{{
			"address": address,
			"port":    port,
			"users":   []map[string]any{user},
		}},
	}
	q := url.Values{}
	copyValue(q, "type", stringFromMap(node, "net"))
	copyValue(q, "headerType", stringFromMap(node, "type"))
	copyValue(q, "host", stringFromMap(node, "host"))
	copyValue(q, "path", stringFromMap(node, "path"))
	copyValue(q, "security", stringFromMap(node, "tls"))
	copyValue(q, "sni", stringFromMap(node, "sni"))
	copyValue(q, "fp", stringFromMap(node, "fp"))
	return taggedOutbound("vmess", settings, xrayStreamSettings(q)), nil
}

func buildVLESSOutbound(u *url.URL) (map[string]any, error) {
	port := portFromURL(u)
	id := u.User.Username()
	if u.Hostname() == "" || port <= 0 || id == "" {
		return nil, errors.New("vless node missing address, port or uuid")
	}
	q := u.Query()
	user := map[string]any{"id": id, "encryption": firstQuery(q, "encryption")}
	if user["encryption"] == "" {
		user["encryption"] = "none"
	}
	if flow := firstQuery(q, "flow"); flow != "" {
		user["flow"] = flow
	}
	settings := map[string]any{
		"vnext": []map[string]any{{
			"address": u.Hostname(),
			"port":    port,
			"users":   []map[string]any{user},
		}},
	}
	return taggedOutbound("vless", settings, xrayStreamSettings(q)), nil
}

func buildTrojanOutbound(u *url.URL) (map[string]any, error) {
	port := portFromURL(u)
	password := u.User.Username()
	if u.Hostname() == "" || port <= 0 || password == "" {
		return nil, errors.New("trojan node missing address, port or password")
	}
	server := map[string]any{"address": u.Hostname(), "port": port, "password": password}
	if flow := firstQuery(u.Query(), "flow"); flow != "" {
		server["flow"] = flow
	}
	settings := map[string]any{"servers": []map[string]any{server}}
	return taggedOutbound("trojan", settings, xrayStreamSettings(u.Query())), nil
}

func buildShadowsocksOutbound(raw string) (map[string]any, error) {
	method, password, host, port, err := parseShadowsocksShare(raw)
	if err != nil {
		return nil, err
	}
	settings := map[string]any{
		"servers": []map[string]any{{
			"address":  host,
			"port":     port,
			"method":   method,
			"password": password,
		}},
	}
	return taggedOutbound("shadowsocks", settings, nil), nil
}

func taggedOutbound(protocol string, settings map[string]any, stream map[string]any) map[string]any {
	out := map[string]any{
		"tag":      "sub2api-out",
		"protocol": protocol,
		"settings": settings,
	}
	if len(stream) > 0 {
		out["streamSettings"] = stream
	}
	return out
}

func xrayStreamSettings(q url.Values) map[string]any {
	network := firstQuery(q, "type", "network")
	if network == "" {
		network = "tcp"
	}
	security := firstQuery(q, "security", "tls")
	if security == "none" {
		security = ""
	}
	out := map[string]any{"network": network}
	if security != "" {
		out["security"] = security
	}
	switch security {
	case "tls":
		if tls := tlsSettings(q); len(tls) > 0 {
			out["tlsSettings"] = tls
		}
	case "reality":
		if reality := realitySettings(q); len(reality) > 0 {
			out["realitySettings"] = reality
		}
	}
	switch network {
	case "ws", "websocket":
		out["network"] = "ws"
		ws := map[string]any{}
		if path := firstQuery(q, "path"); path != "" {
			ws["path"] = path
		}
		if host := firstQuery(q, "host"); host != "" {
			ws["headers"] = map[string]any{"Host": host}
		}
		out["wsSettings"] = ws
	case "grpc":
		grpc := map[string]any{}
		if serviceName := firstQuery(q, "serviceName", "service_name", "path"); serviceName != "" {
			grpc["serviceName"] = strings.TrimPrefix(serviceName, "/")
		}
		out["grpcSettings"] = grpc
	case "httpupgrade":
		httpUpgrade := map[string]any{}
		if path := firstQuery(q, "path"); path != "" {
			httpUpgrade["path"] = path
		}
		if host := firstQuery(q, "host"); host != "" {
			httpUpgrade["host"] = host
		}
		out["httpupgradeSettings"] = httpUpgrade
	case "tcp":
		headerType := firstQuery(q, "headerType", "header")
		if headerType != "" && headerType != "none" {
			out["tcpSettings"] = map[string]any{"header": map[string]any{"type": headerType}}
		}
	}
	return out
}

func tlsSettings(q url.Values) map[string]any {
	out := map[string]any{}
	if sni := firstQuery(q, "sni", "serverName", "peer"); sni != "" {
		out["serverName"] = sni
	}
	if fp := firstQuery(q, "fp", "fingerprint"); fp != "" {
		out["fingerprint"] = fp
	}
	if alpn := firstQuery(q, "alpn"); alpn != "" {
		out["alpn"] = splitCSV(alpn)
	}
	if allow := firstQuery(q, "allowInsecure", "allow_insecure"); allow != "" {
		out["allowInsecure"] = boolString(allow)
	}
	return out
}

func realitySettings(q url.Values) map[string]any {
	out := tlsSettings(q)
	if publicKey := firstQuery(q, "pbk", "publicKey"); publicKey != "" {
		out["publicKey"] = publicKey
	}
	if shortID := firstQuery(q, "sid", "shortId", "shortID"); shortID != "" {
		out["shortId"] = shortID
	}
	if spiderX := firstQuery(q, "spx", "spiderX"); spiderX != "" {
		out["spiderX"] = spiderX
	}
	return out
}

func parseShadowsocksShare(raw string) (method, password, host string, port int, err error) {
	withoutScheme := strings.TrimPrefix(strings.TrimSpace(raw), "ss://")
	mainPart := withoutScheme
	if idx := strings.IndexAny(mainPart, "?#"); idx >= 0 {
		mainPart = mainPart[:idx]
	}
	if !strings.Contains(mainPart, "@") {
		decoded, ok := decodeShareBase64(mainPart)
		if !ok {
			return "", "", "", 0, errors.New("invalid shadowsocks payload")
		}
		mainPart = decoded
	}
	u, err := url.Parse("ss://" + mainPart)
	if err != nil {
		return "", "", "", 0, err
	}
	host = u.Hostname()
	port = portFromURL(u)
	userInfo := u.User.String()
	if decoded, ok := decodeShareBase64(userInfo); ok && strings.Contains(decoded, ":") {
		userInfo = decoded
	}
	parts := strings.SplitN(userInfo, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || host == "" || port <= 0 {
		return "", "", "", 0, errors.New("shadowsocks node missing method, password, host or port")
	}
	method, password = parts[0], parts[1]
	return method, password, host, port, nil
}

func decodeShareBase64(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	encodings := []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	}
	for _, enc := range encodings {
		if decoded, err := enc.DecodeString(s); err == nil && len(decoded) > 0 {
			return string(decoded), true
		}
	}
	if padded := padBase64(s); padded != s {
		for _, enc := range []*base64.Encoding{base64.URLEncoding, base64.StdEncoding} {
			if decoded, err := enc.DecodeString(padded); err == nil && len(decoded) > 0 {
				return string(decoded), true
			}
		}
	}
	return "", false
}

func padBase64(s string) string {
	if rem := len(s) % 4; rem != 0 {
		return s + strings.Repeat("=", 4-rem)
	}
	return s
}

func firstQuery(q url.Values, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(q.Get(key)); value != "" {
			if decoded, err := url.QueryUnescape(value); err == nil {
				return decoded
			}
			return value
		}
	}
	return ""
}

func copyValue(q url.Values, key, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		q.Set(key, value)
	}
}

func splitCSV(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' })
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func boolString(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "y":
		return true
	default:
		return false
	}
}

func stringFromMap(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return t.String()
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", t))
	}
}

func intFromAnyValue(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case json.Number:
		i, _ := t.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(t))
		return i
	default:
		return 0
	}
}

func portFromURL(u *url.URL) int {
	if u == nil {
		return 0
	}
	port, _ := strconv.Atoi(u.Port())
	return port
}
