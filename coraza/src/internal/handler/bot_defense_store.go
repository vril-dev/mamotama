package handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	botDefenseModeSuspicious = "suspicious"
	botDefenseModeAlways     = "always"
)

type botDefenseConfig struct {
	Enabled              bool     `json:"enabled"`
	Mode                 string   `json:"mode"`
	PathPrefixes         []string `json:"path_prefixes,omitempty"`
	ExemptCIDRs          []string `json:"exempt_cidrs,omitempty"`
	SuspiciousUserAgents []string `json:"suspicious_user_agents,omitempty"`
	ChallengeCookieName  string   `json:"challenge_cookie_name,omitempty"`
	ChallengeSecret      string   `json:"challenge_secret,omitempty"`
	ChallengeTTLSeconds  int      `json:"challenge_ttl_seconds"`
	ChallengeStatusCode  int      `json:"challenge_status_code"`
}

type runtimeBotDefenseConfig struct {
	Raw             botDefenseConfig
	Mode            string
	PathPrefixes    []string
	ExemptPrefixes  []netip.Prefix
	SuspiciousUA    []string
	CookieName      string
	Secret          []byte
	ChallengeTTL    time.Duration
	ChallengeStatus int
	EphemeralSecret bool
}

type botDefenseDecision struct {
	Allowed    bool
	Status     int
	Mode       string
	CookieName string
	Token      string
	TTLSeconds int
}

var (
	botDefenseMu      sync.RWMutex
	botDefensePath    string
	botDefenseRuntime *runtimeBotDefenseConfig
)

func InitBotDefense(path string) error {
	target := strings.TrimSpace(path)
	if target == "" {
		return fmt.Errorf("bot defense path is empty")
	}
	if err := ensureBotDefenseFile(target); err != nil {
		return err
	}

	botDefenseMu.Lock()
	botDefensePath = target
	botDefenseMu.Unlock()

	return ReloadBotDefense()
}

func GetBotDefensePath() string {
	botDefenseMu.RLock()
	defer botDefenseMu.RUnlock()
	return botDefensePath
}

func GetBotDefenseConfig() botDefenseConfig {
	botDefenseMu.RLock()
	defer botDefenseMu.RUnlock()
	if botDefenseRuntime == nil {
		return botDefenseConfig{}
	}
	return botDefenseRuntime.Raw
}

func ReloadBotDefense() error {
	path := GetBotDefensePath()
	if path == "" {
		return fmt.Errorf("bot defense path is empty")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	rt, err := buildBotDefenseRuntimeFromRaw(raw)
	if err != nil {
		return err
	}

	botDefenseMu.Lock()
	botDefenseRuntime = rt
	botDefenseMu.Unlock()

	if rt.EphemeralSecret && rt.Raw.Enabled {
		log.Printf("[BOT_DEFENSE][WARN] challenge_secret is empty; generated ephemeral secret for this process")
	}

	return nil
}

func ValidateBotDefenseRaw(raw string) (*runtimeBotDefenseConfig, error) {
	return buildBotDefenseRuntimeFromRaw([]byte(raw))
}

func EvaluateBotDefense(r *http.Request, clientIP string, now time.Time) botDefenseDecision {
	rt := currentBotDefenseRuntime()
	if rt == nil || !rt.Raw.Enabled {
		return botDefenseDecision{Allowed: true}
	}
	if r == nil || r.URL == nil {
		return botDefenseDecision{Allowed: true}
	}
	if r.Method != http.MethodGet {
		return botDefenseDecision{Allowed: true}
	}

	reqPath := strings.TrimSpace(r.URL.Path)
	if reqPath == "" {
		reqPath = "/"
	}
	if !pathMatchesAnyPrefix(rt.PathPrefixes, reqPath) {
		return botDefenseDecision{Allowed: true}
	}

	clientIP = normalizeClientIP(clientIP)
	if isBotDefenseExemptIP(rt, clientIP) {
		return botDefenseDecision{Allowed: true}
	}

	userAgent := r.UserAgent()
	if rt.Mode == botDefenseModeSuspicious && !isSuspiciousUserAgent(rt.SuspiciousUA, userAgent) {
		return botDefenseDecision{Allowed: true}
	}
	if hasValidBotDefenseCookie(rt, r, clientIP, userAgent, now.UTC()) {
		return botDefenseDecision{Allowed: true}
	}

	ttlSeconds := int(rt.ChallengeTTL.Seconds())
	if ttlSeconds < 1 {
		ttlSeconds = 1
	}
	return botDefenseDecision{
		Allowed:    false,
		Status:     rt.ChallengeStatus,
		Mode:       rt.Mode,
		CookieName: rt.CookieName,
		Token:      issueBotDefenseToken(rt, clientIP, userAgent, now.UTC()),
		TTLSeconds: ttlSeconds,
	}
}

func WriteBotDefenseChallenge(w http.ResponseWriter, r *http.Request, d botDefenseDecision) {
	status := d.Status
	if status == 0 {
		status = http.StatusTooManyRequests
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Mamotama-Bot-Challenge", "required")

	if !acceptsHTML(r.Header.Get("Accept")) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"error":"bot challenge required"}`))
		return
	}

	maxAge := d.TTLSeconds
	if maxAge < 1 {
		maxAge = 1
	}
	html := fmt.Sprintf(`<!doctype html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Challenge Required</title></head>
<body>
<p>Verifying browser...</p>
<script>
(() => {
  const token = %q;
  const cookieName = %q;
  document.cookie = cookieName + "=" + token + "; Path=/; Max-Age=%d; SameSite=Lax";
  window.location.replace(window.location.href);
})();
</script>
<noscript>JavaScript is required to continue.</noscript>
</body></html>`, d.Token, d.CookieName, maxAge)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(html))
}

func currentBotDefenseRuntime() *runtimeBotDefenseConfig {
	botDefenseMu.RLock()
	defer botDefenseMu.RUnlock()
	return botDefenseRuntime
}

func buildBotDefenseRuntimeFromRaw(raw []byte) (*runtimeBotDefenseConfig, error) {
	var cfg botDefenseConfig
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}

	cfg.Mode = normalizeBotDefenseMode(cfg.Mode)
	if cfg.Mode == "" {
		cfg.Mode = botDefenseModeSuspicious
	}
	if cfg.Mode != botDefenseModeSuspicious && cfg.Mode != botDefenseModeAlways {
		return nil, fmt.Errorf("mode must be suspicious|always")
	}

	cfg.PathPrefixes = normalizePathPrefixes(cfg.PathPrefixes)
	if len(cfg.PathPrefixes) == 0 {
		cfg.PathPrefixes = []string{"/"}
	}

	exempt, err := normalizeBotDefenseCIDRs(cfg.ExemptCIDRs)
	if err != nil {
		return nil, err
	}
	cfg.ExemptCIDRs = make([]string, 0, len(exempt))
	for _, pfx := range exempt {
		cfg.ExemptCIDRs = append(cfg.ExemptCIDRs, pfx.String())
	}

	cfg.SuspiciousUserAgents = normalizeLowerStringList(cfg.SuspiciousUserAgents)
	if len(cfg.SuspiciousUserAgents) == 0 {
		cfg.SuspiciousUserAgents = defaultSuspiciousUserAgents()
	}

	cfg.ChallengeCookieName = strings.TrimSpace(cfg.ChallengeCookieName)
	if cfg.ChallengeCookieName == "" {
		cfg.ChallengeCookieName = "__mamotama_bot_ok"
	}
	if !isValidCookieName(cfg.ChallengeCookieName) {
		return nil, fmt.Errorf("challenge_cookie_name is invalid")
	}

	if cfg.ChallengeTTLSeconds <= 0 {
		cfg.ChallengeTTLSeconds = 24 * 60 * 60
	}
	if cfg.ChallengeStatusCode == 0 {
		cfg.ChallengeStatusCode = http.StatusTooManyRequests
	}
	if cfg.ChallengeStatusCode < 400 || cfg.ChallengeStatusCode > 599 {
		return nil, fmt.Errorf("challenge_status_code must be 400-599")
	}

	secret := []byte(strings.TrimSpace(cfg.ChallengeSecret))
	ephemeral := false
	if len(secret) == 0 {
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			secret = []byte("mamotama-bot-defense-ephemeral")
		}
		ephemeral = true
	}

	return &runtimeBotDefenseConfig{
		Raw:             cfg,
		Mode:            cfg.Mode,
		PathPrefixes:    append([]string(nil), cfg.PathPrefixes...),
		ExemptPrefixes:  exempt,
		SuspiciousUA:    append([]string(nil), cfg.SuspiciousUserAgents...),
		CookieName:      cfg.ChallengeCookieName,
		Secret:          secret,
		ChallengeTTL:    time.Duration(cfg.ChallengeTTLSeconds) * time.Second,
		ChallengeStatus: cfg.ChallengeStatusCode,
		EphemeralSecret: ephemeral,
	}, nil
}

func normalizeBotDefenseMode(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func normalizePathPrefixes(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if !strings.HasPrefix(v, "/") {
			v = "/" + v
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func normalizeLowerStringList(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		v := strings.ToLower(strings.TrimSpace(raw))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func normalizeBotDefenseCIDRs(in []string) ([]netip.Prefix, error) {
	out := make([]netip.Prefix, 0, len(in))
	seen := map[string]struct{}{}
	for i, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}

		if pfx, err := netip.ParsePrefix(v); err == nil {
			if _, ok := seen[pfx.String()]; ok {
				continue
			}
			seen[pfx.String()] = struct{}{}
			out = append(out, pfx)
			continue
		}

		addr, err := netip.ParseAddr(v)
		if err != nil {
			return nil, fmt.Errorf("exempt_cidrs[%d]: invalid address/CIDR: %s", i, v)
		}
		bits := 32
		if addr.Is6() {
			bits = 128
		}
		pfx := netip.PrefixFrom(addr, bits)
		if _, ok := seen[pfx.String()]; ok {
			continue
		}
		seen[pfx.String()] = struct{}{}
		out = append(out, pfx)
	}
	return out, nil
}

func defaultSuspiciousUserAgents() []string {
	return []string{
		"curl",
		"wget",
		"python-requests",
		"python-urllib",
		"go-http-client",
		"libwww-perl",
		"scrapy",
		"sqlmap",
		"nikto",
		"nmap",
		"masscan",
	}
}

func isValidCookieName(v string) bool {
	if strings.TrimSpace(v) == "" {
		return false
	}
	for i := 0; i < len(v); i++ {
		ch := v[i]
		if ch <= 0x20 || ch >= 0x7f {
			return false
		}
		switch ch {
		case '(', ')', '<', '>', '@', ',', ';', ':', '\\', '"', '/', '[', ']', '?', '=', '{', '}', ' ':
			return false
		}
	}
	return true
}

func isBotDefenseExemptIP(rt *runtimeBotDefenseConfig, ipStr string) bool {
	if ipStr == "" || rt == nil || len(rt.ExemptPrefixes) == 0 {
		return false
	}
	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return false
	}
	for _, pfx := range rt.ExemptPrefixes {
		if pfx.Contains(ip) {
			return true
		}
	}
	return false
}

func isSuspiciousUserAgent(list []string, ua string) bool {
	v := strings.ToLower(strings.TrimSpace(ua))
	if v == "" {
		return true
	}
	for _, needle := range list {
		if needle != "" && strings.Contains(v, needle) {
			return true
		}
	}
	return false
}

func hasValidBotDefenseCookie(rt *runtimeBotDefenseConfig, r *http.Request, ip, userAgent string, now time.Time) bool {
	if rt == nil || r == nil {
		return false
	}
	c, err := r.Cookie(rt.CookieName)
	if err != nil {
		return false
	}
	return verifyBotDefenseToken(rt, c.Value, ip, userAgent, now)
}

func issueBotDefenseToken(rt *runtimeBotDefenseConfig, ip, userAgent string, now time.Time) string {
	exp := now.Add(rt.ChallengeTTL).Unix()
	payload := strconv.FormatInt(exp, 10)
	sig := signBotDefenseToken(rt, ip, userAgent, payload)
	return payload + "." + sig
}

func verifyBotDefenseToken(rt *runtimeBotDefenseConfig, token, ip, userAgent string, now time.Time) bool {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 {
		return false
	}

	expUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || expUnix <= 0 {
		return false
	}
	if now.Unix() > expUnix {
		return false
	}

	expected := signBotDefenseToken(rt, ip, userAgent, parts[0])
	return subtleConstantTimeHexEqual(parts[1], expected)
}

func signBotDefenseToken(rt *runtimeBotDefenseConfig, ip, userAgent, payload string) string {
	mac := hmac.New(sha256.New, rt.Secret)
	_, _ = mac.Write([]byte(strings.TrimSpace(ip)))
	_, _ = mac.Write([]byte{'\n'})
	_, _ = mac.Write([]byte(strings.ToLower(strings.TrimSpace(userAgent))))
	_, _ = mac.Write([]byte{'\n'})
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func subtleConstantTimeHexEqual(a, b string) bool {
	ab, errA := hex.DecodeString(strings.TrimSpace(a))
	bb, errB := hex.DecodeString(strings.TrimSpace(b))
	if errA != nil || errB != nil || len(ab) != len(bb) {
		return false
	}
	return hmac.Equal(ab, bb)
}

func acceptsHTML(rawAccept string) bool {
	v := strings.ToLower(strings.TrimSpace(rawAccept))
	if v == "" {
		return false
	}
	return strings.Contains(v, "text/html") || strings.Contains(v, "*/*")
}

func pathMatchesAnyPrefix(prefixes []string, path string) bool {
	if path == "" {
		path = "/"
	}
	for _, pfx := range prefixes {
		if pfx == "/" || strings.HasPrefix(path, pfx) {
			return true
		}
	}
	return false
}

func ensureBotDefenseFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	const defaultRaw = `{
  "enabled": true,
  "mode": "suspicious",
  "path_prefixes": ["/"],
  "exempt_cidrs": [
    "127.0.0.1/32",
    "::1/128",
    "10.0.0.0/8",
    "172.16.0.0/12",
    "192.168.0.0/16",
    "fc00::/7"
  ],
  "suspicious_user_agents": [
    "curl",
    "wget",
    "python-requests",
    "python-urllib",
    "python-httpx",
    "go-http-client",
    "aiohttp",
    "libwww-perl",
    "scrapy",
    "headless",
    "selenium",
    "puppeteer",
    "playwright",
    "sqlmap",
    "nikto",
    "nmap",
    "masscan"
  ],
  "challenge_cookie_name": "__mamotama_bot_ok",
  "challenge_secret": "",
  "challenge_ttl_seconds": 21600,
  "challenge_status_code": 429
}
`
	return os.WriteFile(path, []byte(defaultRaw), 0o644)
}
