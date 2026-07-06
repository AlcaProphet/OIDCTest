package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ============================================================
// 数据结构
// ============================================================

// OIDCConfig OIDC 配置
type OIDCConfig struct {
	Issuer       string `json:"issuer"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Scopes       string `json:"scopes"`
	Flow         string `json:"flow"`
	BaseURL      string `json:"base_url"`
}

// DebugStep 调试步骤记录
type DebugStep struct {
	Timestamp  time.Time `json:"timestamp"`
	Name       string    `json:"name"`
	Method     string    `json:"method"`
	URL        string    `json:"url"`
	ReqBody    string    `json:"req_body"`
	StatusCode int       `json:"status_code"`
	RespBody   string    `json:"resp_body"`
	Error      string    `json:"error"`
	DurationMs int64     `json:"duration_ms"`
}

// TokenResult Token 结果
type TokenResult struct {
	AccessToken    string                 `json:"access_token"`
	TokenType      string                 `json:"token_type"`
	ExpiresIn      int                    `json:"expires_in"`
	RefreshToken   string                 `json:"refresh_token"`
	IDToken        string                 `json:"id_token"`
	IDTokenHeader  map[string]interface{} `json:"id_token_header"`
	IDTokenClaims  map[string]interface{} `json:"id_token_claims"`
	UserInfo       map[string]interface{} `json:"user_info"`
	AccessTokenJWT map[string]interface{} `json:"access_token_jwt,omitempty"`
}

// Session 会话
type Session struct {
	ID           string       `json:"id"`
	Flow         string       `json:"flow"`
	OIDCConfig   OIDCConfig   `json:"config"`
	State        string       `json:"state"`
	CodeVerifier string       `json:"code_verifier"`
	DebugSteps   []DebugStep  `json:"steps"`
	TokenResult  *TokenResult `json:"result"`
	CreatedAt    time.Time    `json:"created_at"`
}

// ============================================================
// 应用结构
// ============================================================

type App struct {
	db   *sql.DB
	tmpl *template.Template
}

// PageData 通用页面数据
type PageData struct {
	Config      *OIDCConfig
	Session     *Session
	Error       string
	AutoBaseURL string
}

// ============================================================
// 模板函数
// ============================================================

var funcMap = template.FuncMap{
	"formatJSON": func(v interface{}) string {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	},
	"add": func(a, b int) int {
		return a + b
	},
}

// ============================================================
// 辅助函数
// ============================================================

// detectBaseURL 从请求头检测 Base URL
func detectBaseURL(r *http.Request) string {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "http"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return proto + "://" + host
}

// getEffectiveBaseURL 获取实际使用的 Base URL（手动覆盖优先）
func getEffectiveBaseURL(r *http.Request, config *OIDCConfig) string {
	if config != nil && config.BaseURL != "" {
		return strings.TrimRight(config.BaseURL, "/")
	}
	return strings.TrimRight(detectBaseURL(r), "/")
}

// setSessionCookie 设置会话 Cookie
func setSessionCookie(w http.ResponseWriter, sid string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "sid",
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400, // 24 小时
	})
}

// clearSessionCookie 清除会话 Cookie
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "sid",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// getSessionCookie 从请求中读取 sid Cookie
func getSessionCookie(r *http.Request) string {
	cookie, err := r.Cookie("sid")
	if err != nil {
		return ""
	}
	return cookie.Value
}

// ============================================================
// HTTP 处理器
// ============================================================

// handleIndex 首页：配置表单 / 操作按钮
func (app *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	config, _ := GetConfig(app.db)
	autoBaseURL := detectBaseURL(r)

	// 如果 URL 带有 ?edit=1 参数，强制显示配置表单
	if r.URL.Query().Get("edit") == "1" {
		data := PageData{
			Config:      nil,
			AutoBaseURL: autoBaseURL,
		}
		app.render(w, "index.html", data)
		return
	}

	// 如果有配置且 BaseURL 为空，自动填充检测到的值
	if config != nil && config.BaseURL == "" {
		config.BaseURL = autoBaseURL
	}

	data := PageData{
		Config:      config,
		AutoBaseURL: autoBaseURL,
	}
	app.render(w, "index.html", data)
}

// handleConfig 保存 OIDC 配置
func (app *App) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		app.renderError(w, "解析表单失败: "+err.Error())
		return
	}

	// 必填非空检查
	issuer := strings.TrimSpace(r.FormValue("issuer"))
	clientID := strings.TrimSpace(r.FormValue("client_id"))
	clientSecret := strings.TrimSpace(r.FormValue("client_secret"))

	if issuer == "" || clientID == "" || clientSecret == "" {
		app.renderError(w, "Issuer、Client ID 和 Client Secret 为必填项")
		return
	}

	scopes := strings.TrimSpace(r.FormValue("scopes"))
	if scopes == "" {
		scopes = "openid profile email"
	}

	flow := r.FormValue("flow")
	if flow != "authcode-pkce" && flow != "authcode" {
		flow = "authcode-pkce"
	}

	baseURL := strings.TrimSpace(r.FormValue("base_url"))
	if baseURL == "" {
		baseURL = detectBaseURL(r)
	}

	config := OIDCConfig{
		Issuer:       issuer,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       scopes,
		Flow:         flow,
		BaseURL:      baseURL,
	}

	if err := SaveConfig(app.db, config); err != nil {
		app.renderError(w, "保存配置失败: "+err.Error())
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleLogin 发起 OIDC 登录
func (app *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	config, err := GetConfig(app.db)
	if err != nil || config == nil {
		app.renderError(w, "请先完成 OIDC 配置")
		return
	}

	baseURL := getEffectiveBaseURL(r, config)
	redirectURI := baseURL + "/callback"

	// Discovery
	var steps []DebugStep
	endpoints, err := Discover(config.Issuer, &steps)
	if err != nil {
		app.renderError(w, "OIDC Discovery 失败: "+err.Error())
		return
	}

	// 生成 state
	state, err := GenerateRandomString(32)
	if err != nil {
		app.renderError(w, "生成 state 失败: "+err.Error())
		return
	}

	// 生成 PKCE 参数 (仅 authcode-pkce 流程)
	var codeVerifier, codeChallenge string
	if config.Flow == "authcode-pkce" {
		codeVerifier, err = GenerateRandomString(32)
		if err != nil {
			app.renderError(w, "生成 code_verifier 失败: "+err.Error())
			return
		}
		codeChallenge = GenerateCodeChallenge(codeVerifier)
	}

	// 创建会话
	sessionID, err := GenerateRandomString(32)
	if err != nil {
		app.renderError(w, "生成 session ID 失败: "+err.Error())
		return
	}

	sess := &Session{
		ID:           sessionID,
		Flow:         config.Flow,
		OIDCConfig:   *config,
		State:        state,
		CodeVerifier: codeVerifier,
		DebugSteps:   steps,
		CreatedAt:    time.Now(),
	}

	if err := CreateSession(app.db, sess); err != nil {
		app.renderError(w, "创建会话失败: "+err.Error())
		return
	}

	// 设置 Cookie
	setSessionCookie(w, sessionID)

	// 构造授权 URL 并跳转
	authURL := BuildAuthURL(
		endpoints.AuthorizationEndpoint,
		config.ClientID,
		redirectURI,
		state,
		codeChallenge,
		config.Scopes,
		config.Flow,
	)

	http.Redirect(w, r, authURL, http.StatusSeeOther)
}

// handleCallback OIDC 回调处理
func (app *App) handleCallback(w http.ResponseWriter, r *http.Request) {
	sid := getSessionCookie(r)
	if sid == "" {
		app.renderError(w, "未找到会话 Cookie，请重新登录")
		return
	}

	sess, err := GetSession(app.db, sid)
	if err != nil || sess == nil {
		app.renderError(w, "会话不存在或已过期，请重新登录")
		return
	}

	// 获取回调参数
	code := r.URL.Query().Get("code")
	returnedState := r.URL.Query().Get("state")
	errParam := r.URL.Query().Get("error")
	errDesc := r.URL.Query().Get("error_description")

	// 错误处理：Keycloak 可能通过 redirect_uri + error 参数返回错误
	if errParam != "" {
		errorMsg := "OIDC 错误: " + errParam
		if errDesc != "" {
			errorMsg += " — " + errDesc
		}
		sess.DebugSteps = append(sess.DebugSteps, DebugStep{
			Timestamp: time.Now(),
			Name:      "回调错误",
			Method:    "GET",
			URL:       r.URL.String(),
			Error:     errorMsg,
		})
		// 更新 session 的错误步骤以便展示
		UpdateSessionResult(app.db, sid, TokenResult{}, sess.DebugSteps)
		http.Redirect(w, r, "/result", http.StatusSeeOther)
		return
	}

	if code == "" {
		app.renderError(w, "未收到授权码 (code)")
		return
	}

	// 验证 state
	if returnedState == "" || returnedState != sess.State {
		sess.DebugSteps = append(sess.DebugSteps, DebugStep{
			Timestamp: time.Now(),
			Name:      "State 验证",
			Method:    "GET",
			URL:       r.URL.String(),
			Error:     fmt.Sprintf("State 不匹配: 期望 %s, 收到 %s", sess.State, returnedState),
		})
		UpdateSessionResult(app.db, sid, TokenResult{}, sess.DebugSteps)
		http.Redirect(w, r, "/result", http.StatusSeeOther)
		return
	}

	sess.DebugSteps = append(sess.DebugSteps, DebugStep{
		Timestamp: time.Now(),
		Name:      "回调接收",
		Method:    "GET",
		URL:       r.URL.Path + "?code=***&state=" + returnedState[:8] + "...",
	})

	config := sess.OIDCConfig
	baseURL := getEffectiveBaseURL(r, &config)
	redirectURI := baseURL + "/callback"

	// Discovery（回调中再次调用以确保 token endpoint 正确）
	endpoints, err := Discover(config.Issuer, &sess.DebugSteps)
	if err != nil {
		UpdateSessionResult(app.db, sid, TokenResult{}, sess.DebugSteps)
		http.Redirect(w, r, "/result", http.StatusSeeOther)
		return
	}

	// 交换 code
	tokenResp, err := ExchangeCode(
		endpoints.TokenEndpoint,
		config.ClientID,
		config.ClientSecret,
		redirectURI,
		code,
		sess.CodeVerifier,
		&sess.DebugSteps,
	)
	if err != nil {
		UpdateSessionResult(app.db, sid, TokenResult{}, sess.DebugSteps)
		http.Redirect(w, r, "/result", http.StatusSeeOther)
		return
	}

	// 解码 ID Token
	var idTokenHeader, idTokenClaims map[string]interface{}
	if tokenResp.IDToken != "" {
		idTokenHeader, idTokenClaims, _ = DecodeJWT(tokenResp.IDToken)
		sess.DebugSteps = append(sess.DebugSteps, DebugStep{
			Timestamp: time.Now(),
			Name:      "ID Token 解码",
			Method:    "-",
			URL:       "-",
			StatusCode: 200,
			RespBody:   "成功解码 ID Token (Header + Payload)",
		})
	}

	// 尝试解码 Access Token (如果是 JWT)
	var accessTokenJWT map[string]interface{}
	if strings.Count(tokenResp.AccessToken, ".") >= 2 {
		_, accessTokenJWT, _ = DecodeJWT(tokenResp.AccessToken)
	}

	// 获取 UserInfo
	var userInfo map[string]interface{}
	if endpoints.UserinfoEndpoint != "" {
		userInfo, err = GetUserInfo(endpoints.UserinfoEndpoint, tokenResp.AccessToken, &sess.DebugSteps)
		if err != nil {
			// userinfo 失败不阻断流程
			sess.DebugSteps = append(sess.DebugSteps, DebugStep{
				Timestamp: time.Now(),
				Name:      "UserInfo 结果",
				Method:    "-",
				URL:       "-",
				Error:     "UserInfo 获取失败: " + err.Error(),
			})
		}
	}

	// 保存 Token 结果
	result := TokenResult{
		AccessToken:    tokenResp.AccessToken,
		TokenType:      tokenResp.TokenType,
		ExpiresIn:      tokenResp.ExpiresIn,
		RefreshToken:   tokenResp.RefreshToken,
		IDToken:        tokenResp.IDToken,
		IDTokenHeader:  idTokenHeader,
		IDTokenClaims:  idTokenClaims,
		UserInfo:       userInfo,
		AccessTokenJWT: accessTokenJWT,
	}

	if err := UpdateSessionResult(app.db, sid, result, sess.DebugSteps); err != nil {
		app.renderError(w, "保存结果失败: "+err.Error())
		return
	}

	http.Redirect(w, r, "/result", http.StatusSeeOther)
}

// handleResult 结果展示页
func (app *App) handleResult(w http.ResponseWriter, r *http.Request) {
	sid := getSessionCookie(r)
	if sid == "" {
		app.render(w, "result.html", PageData{Error: "暂无结果，请先执行登录操作。"})
		return
	}

	sess, err := GetSession(app.db, sid)
	if err != nil || sess == nil {
		app.render(w, "result.html", PageData{Error: "会话不存在或已过期。"})
		return
	}

	data := PageData{
		Session: sess,
		Config:  &sess.OIDCConfig,
	}
	app.render(w, "result.html", data)
}

// handleLogout 退出登录
func (app *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	sid := getSessionCookie(r)
	var idToken string
	var endSessionEndpoint string
	var logoutConfig OIDCConfig

	if sid != "" {
		sess, _ := GetSession(app.db, sid)
		if sess != nil {
			logoutConfig = sess.OIDCConfig
			if sess.TokenResult != nil {
				idToken = sess.TokenResult.IDToken
			}

			// 尝试 Discovery 获取 end_session_endpoint
			var steps []DebugStep
			endpoints, err := Discover(sess.OIDCConfig.Issuer, &steps)
			if err == nil {
				endSessionEndpoint = endpoints.EndSessionEndpoint
			}
		}

		// 删除本地会话
		DeleteSession(app.db, sid)
	}

	// 清除 Cookie
	clearSessionCookie(w)

	// 构造 Keycloak 退出 URL
	if endSessionEndpoint != "" {
		baseURL := getEffectiveBaseURL(r, &logoutConfig)
		logoutURL := endSessionEndpoint + "?post_logout_redirect_uri=" + url.QueryEscape(baseURL)
		if idToken != "" {
			logoutURL += "&id_token_hint=" + url.QueryEscape(idToken)
		}
		http.Redirect(w, r, logoutURL, http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleClientCredentials Client Credentials 流程
func (app *App) handleClientCredentials(w http.ResponseWriter, r *http.Request) {
	config, err := GetConfig(app.db)
	if err != nil || config == nil {
		app.renderError(w, "请先完成 OIDC 配置")
		return
	}

	// Discovery
	var steps []DebugStep
	endpoints, err := Discover(config.Issuer, &steps)
	if err != nil {
		app.renderError(w, "OIDC Discovery 失败: "+err.Error())
		return
	}

	// 执行 Client Credentials
	tokenResp, err := ClientCredentials(
		endpoints.TokenEndpoint,
		config.ClientID,
		config.ClientSecret,
		config.Scopes,
		&steps,
	)
	if err != nil {
		app.renderError(w, "Client Credentials 流程失败: "+err.Error())
		return
	}

	// 尝试解码 Access Token (如果是 JWT)
	var accessTokenJWT map[string]interface{}
	if strings.Count(tokenResp.AccessToken, ".") >= 2 {
		_, accessTokenJWT, _ = DecodeJWT(tokenResp.AccessToken)
	}

	// 尝试获取 UserInfo
	var userInfo map[string]interface{}
	if endpoints.UserinfoEndpoint != "" {
		userInfo, _ = GetUserInfo(endpoints.UserinfoEndpoint, tokenResp.AccessToken, &steps)
	}

	// 创建会话存储结果
	sessionID, err := GenerateRandomString(32)
	if err != nil {
		app.renderError(w, "生成 session ID 失败: "+err.Error())
		return
	}

	result := TokenResult{
		AccessToken:    tokenResp.AccessToken,
		TokenType:      tokenResp.TokenType,
		ExpiresIn:      tokenResp.ExpiresIn,
		RefreshToken:   tokenResp.RefreshToken,
		IDToken:        tokenResp.IDToken,
		AccessTokenJWT: accessTokenJWT,
		UserInfo:       userInfo,
	}

	sess := &Session{
		ID:           sessionID,
		Flow:         "client-credentials",
		OIDCConfig:   *config,
		DebugSteps:   steps,
		TokenResult:  &result,
		CreatedAt:    time.Now(),
	}

	if err := CreateSession(app.db, sess); err != nil {
		app.renderError(w, "创建会话失败: "+err.Error())
		return
	}

	// 设置 Cookie
	setSessionCookie(w, sessionID)

	http.Redirect(w, r, "/result", http.StatusSeeOther)
}

// handleDiscover 自动检测 OIDC 端点信息（供配置页 AJAX 调用）
func (app *App) handleDiscover(w http.ResponseWriter, r *http.Request) {
	issuer := strings.TrimSpace(r.URL.Query().Get("issuer"))
	if issuer == "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"error":"缺少 issuer 参数"}`))
		return
	}

	var steps []DebugStep
	endpoints, err := Discover(issuer, &steps)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
			"steps": steps,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"authorization_endpoint": endpoints.AuthorizationEndpoint,
		"token_endpoint":         endpoints.TokenEndpoint,
		"userinfo_endpoint":      endpoints.UserinfoEndpoint,
		"end_session_endpoint":   endpoints.EndSessionEndpoint,
	})
}

// ============================================================
// 模板渲染
// ============================================================

func (app *App) render(w http.ResponseWriter, name string, data PageData) {
	if err := app.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("模板渲染错误: %v", err)
		http.Error(w, "模板渲染错误", http.StatusInternalServerError)
	}
}

func (app *App) renderError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="zh-CN">
<head><meta charset="UTF-8"><title>错误</title></head>
<body style="font-family: sans-serif; padding: 2rem;">
<h1>发生错误</h1>
<p>%s</p>
<a href="/">返回首页</a>
</body>
</html>`, template.HTMLEscapeString(msg))
}

// ============================================================
// 入口
// ============================================================

func main() {
	// 初始化数据库
	db, err := InitDB("./data/playground.db")
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer db.Close()

	// 解析模板
	tmpl := template.New("").Funcs(funcMap)
	tmpl, err = tmpl.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("模板解析失败: %v", err)
	}

	app := &App{db: db, tmpl: tmpl}

	// 注册路由
	http.HandleFunc("/", app.handleIndex)
	http.HandleFunc("/config", app.handleConfig)
	http.HandleFunc("/login", app.handleLogin)
	http.HandleFunc("/callback", app.handleCallback)
	http.HandleFunc("/result", app.handleResult)
	http.HandleFunc("/logout", app.handleLogout)
	http.HandleFunc("/client-credentials", app.handleClientCredentials)
	http.HandleFunc("/discover", app.handleDiscover)

	log.Println("[OK] 服务已启动: http://0.0.0.0:61000")
	if err := http.ListenAndServe(":61000", nil); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
