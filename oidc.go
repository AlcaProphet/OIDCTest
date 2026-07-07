package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OIDCEndpoints 从 Discovery 文档提取的关键端点
type OIDCEndpoints struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
}

// TokenResponse Token 端点的响应结构
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
}

// httpClient 共享 HTTP 客户端，15 秒超时避免 Keycloak 不可达时请求挂起过久
var httpClient = &http.Client{Timeout: 15 * time.Second}

// Discover 调用 issuer/.well-known/openid-configuration 获取端点
func Discover(issuer string, steps *[]DebugStep) (*OIDCEndpoints, error) {
	discoveryURL := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"

	start := time.Now()
	req, _ := http.NewRequest("GET", discoveryURL, nil)
	resp, err := httpClient.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		*steps = append(*steps, DebugStep{
			Timestamp:  time.Now(),
			Name:       "OIDC Discovery",
			Method:     "GET",
			URL:        discoveryURL,
			Error:      err.Error(),
			DurationMs: elapsed.Milliseconds(),
		})
		return nil, fmt.Errorf("discovery 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit

	*steps = append(*steps, DebugStep{
		Timestamp:  time.Now(),
		Name:       "OIDC Discovery",
		Method:     "GET",
		URL:        discoveryURL,
		StatusCode: resp.StatusCode,
		RespBody:   truncateBody(body, 2000),
		DurationMs: elapsed.Milliseconds(),
	})

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery 返回状态码 %d", resp.StatusCode)
	}

	var endpoints OIDCEndpoints
	if err := json.Unmarshal(body, &endpoints); err != nil {
		return nil, fmt.Errorf("解析 discovery 响应失败: %w", err)
	}

	return &endpoints, nil
}

// GenerateRandomString 生成 n 字节随机数，base64url 编码
func GenerateRandomString(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateCodeChallenge 对 verifier 做 SHA256 后 base64url 编码
func GenerateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// BuildAuthURL 构造 OIDC 授权请求 URL
func BuildAuthURL(authEndpoint, clientID, redirectURI, state, codeChallenge, scope, flow string) string {
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", scope)
	params.Set("state", state)

	if flow == "authcode-pkce" && codeChallenge != "" {
		params.Set("code_challenge", codeChallenge)
		params.Set("code_challenge_method", "S256")
	}

	return authEndpoint + "?" + params.Encode()
}

// ExchangeCode 用授权码交换 Token
func ExchangeCode(tokenEndpoint, clientID, clientSecret, redirectURI, code, codeVerifier string, steps *[]DebugStep) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}

	reqBody := form.Encode()
	start := time.Now()
	req, _ := http.NewRequest("POST", tokenEndpoint, strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		*steps = append(*steps, DebugStep{
			Timestamp:  time.Now(),
			Name:       "Token 交换",
			Method:     "POST",
			URL:        tokenEndpoint,
			ReqBody:    maskSecret(reqBody, clientSecret),
			Error:      err.Error(),
			DurationMs: elapsed.Milliseconds(),
		})
		return nil, fmt.Errorf("token 交换请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	*steps = append(*steps, DebugStep{
		Timestamp:  time.Now(),
		Name:       "Token 交换",
		Method:     "POST",
		URL:        tokenEndpoint,
		ReqBody:    maskSecret(reqBody, clientSecret),
		StatusCode: resp.StatusCode,
		RespBody:   truncateBody(body, 2000),
		DurationMs: elapsed.Milliseconds(),
	})

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token 端点返回状态码 %d: %s", resp.StatusCode, truncateBody(body, 500))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析 token 响应失败: %w", err)
	}

	return &tokenResp, nil
}

// GetUserInfo 使用 Access Token 获取用户信息
func GetUserInfo(userinfoEndpoint, accessToken string, steps *[]DebugStep) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", userinfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	start := time.Now()
	resp, err := httpClient.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		*steps = append(*steps, DebugStep{
			Timestamp:  time.Now(),
			Name:       "UserInfo 请求",
			Method:     "GET",
			URL:        userinfoEndpoint,
			Error:      err.Error(),
			DurationMs: elapsed.Milliseconds(),
		})
		return nil, fmt.Errorf("userinfo 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	*steps = append(*steps, DebugStep{
		Timestamp:  time.Now(),
		Name:       "UserInfo 请求",
		Method:     "GET",
		URL:        userinfoEndpoint,
		StatusCode: resp.StatusCode,
		RespBody:   truncateBody(body, 2000),
		DurationMs: elapsed.Milliseconds(),
	})

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo 返回状态码 %d", resp.StatusCode)
	}

	var userInfo map[string]interface{}
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("解析 userinfo 响应失败: %w", err)
	}

	return userInfo, nil
}

// DecodeJWT 解码 JWT — 仅 base64 解码 header 和 payload，不做签名验证
func DecodeJWT(token string) (header map[string]interface{}, payload map[string]interface{}, err error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, nil, fmt.Errorf("无效的 JWT 格式")
	}

	header, err = decodeJWTBase64(parts[0])
	if err != nil {
		return nil, nil, fmt.Errorf("解码 JWT header 失败: %w", err)
	}

	payload, err = decodeJWTBase64(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("解码 JWT payload 失败: %w", err)
	}

	return header, payload, nil
}

// ClientCredentials 执行 Client Credentials 流程
func ClientCredentials(tokenEndpoint, clientID, clientSecret, scope string, steps *[]DebugStep) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("scope", scope)

	reqBody := form.Encode()
	start := time.Now()
	req, _ := http.NewRequest("POST", tokenEndpoint, strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		*steps = append(*steps, DebugStep{
			Timestamp:  time.Now(),
			Name:       "Client Credentials",
			Method:     "POST",
			URL:        tokenEndpoint,
			ReqBody:    maskSecret(reqBody, clientSecret),
			Error:      err.Error(),
			DurationMs: elapsed.Milliseconds(),
		})
		return nil, fmt.Errorf("client credentials 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	*steps = append(*steps, DebugStep{
		Timestamp:  time.Now(),
		Name:       "Client Credentials",
		Method:     "POST",
		URL:        tokenEndpoint,
		ReqBody:    maskSecret(reqBody, clientSecret),
		StatusCode: resp.StatusCode,
		RespBody:   truncateBody(body, 2000),
		DurationMs: elapsed.Milliseconds(),
	})

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("client credentials 返回状态码 %d: %s", resp.StatusCode, truncateBody(body, 500))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析 token 响应失败: %w", err)
	}

	return &tokenResp, nil
}

// decodeJWTBase64 解码 base64url 编码的 JWT 片段
func decodeJWTBase64(s string) (map[string]interface{}, error) {
	// 补齐 base64 填充
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	// 替换 URL-safe 字符
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")

	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(decoded, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// maskSecret 替换请求体中的 client_secret 值为 ***
func maskSecret(body, secret string) string {
	if secret == "" {
		return body
	}
	return strings.ReplaceAll(body, "client_secret="+url.QueryEscape(secret), "client_secret=***")
}

// truncateBody 截断响应体用于日志展示（UTF-8 安全）
func truncateBody(body []byte, maxLen int) string {
	runes := []rune(string(body))
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "...(截断)"
	}
	return string(runes)
}
