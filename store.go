package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

// InitDB 打开 SQLite 数据库并创建表
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 创建表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS kv (
			k TEXT PRIMARY KEY,
			v TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			flow TEXT NOT NULL,
			config TEXT NOT NULL,
			state TEXT NOT NULL DEFAULT '',
			code_verifier TEXT NOT NULL DEFAULT '',
			result TEXT,
			steps TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL
		);
	`)
	if err != nil {
		return nil, fmt.Errorf("创建表失败: %w", err)
	}

	return db, nil
}

// SaveConfig 保存 OIDC 配置到 kv 表
func SaveConfig(db *sql.DB, config OIDCConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`INSERT INTO kv (k, v) VALUES ('config', ?) ON CONFLICT(k) DO UPDATE SET v = ?`,
		string(data), string(data),
	)
	return err
}

// GetConfig 从 kv 表读取 OIDC 配置
func GetConfig(db *sql.DB) (*OIDCConfig, error) {
	var val string
	err := db.QueryRow(`SELECT v FROM kv WHERE k = 'config'`).Scan(&val)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var config OIDCConfig
	err = json.Unmarshal([]byte(val), &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// CreateSession 创建新会话记录
func CreateSession(db *sql.DB, sess *Session) error {
	configJSON, err := json.Marshal(sess.OIDCConfig)
	if err != nil {
		return err
	}
	stepsJSON, err := json.Marshal(sess.DebugSteps)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`INSERT INTO sessions (id, flow, config, state, code_verifier, steps, created_at) 
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.Flow, string(configJSON),
		sess.State, sess.CodeVerifier, string(stepsJSON),
		sess.CreatedAt.Format(time.RFC3339),
	)
	return err
}

// UpdateSessionResult 更新会话的结果和调试步骤
func UpdateSessionResult(db *sql.DB, id string, result TokenResult, steps []DebugStep) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}
	stepsJSON, err := json.Marshal(steps)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`UPDATE sessions SET result = ?, steps = ? WHERE id = ?`,
		string(resultJSON), string(stepsJSON), id,
	)
	return err
}

// GetSession 根据 ID 获取会话
func GetSession(db *sql.DB, id string) (*Session, error) {
	var (
		configJSON    string
		state         string
		codeVerifier  string
		resultJSON    sql.NullString
		stepsJSON     string
		createdAt     string
		flow          string
	)
	err := db.QueryRow(
		`SELECT flow, config, state, code_verifier, result, steps, created_at 
		 FROM sessions WHERE id = ?`, id,
	).Scan(&flow, &configJSON, &state, &codeVerifier, &resultJSON, &stepsJSON, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var config OIDCConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, err
	}

	var steps []DebugStep
	if err := json.Unmarshal([]byte(stepsJSON), &steps); err != nil {
		steps = []DebugStep{}
	}

	var tokenResult *TokenResult
	if resultJSON.Valid && resultJSON.String != "" {
		var tr TokenResult
		if err := json.Unmarshal([]byte(resultJSON.String), &tr); err == nil {
			tokenResult = &tr
		} else {
			log.Printf("解析 session result JSON 失败 [%s]: %v", id, err)
		}
	}

	t, _ := time.Parse(time.RFC3339, createdAt)

	return &Session{
		ID:           id,
		Flow:         flow,
		OIDCConfig:   config,
		State:        state,
		CodeVerifier: codeVerifier,
		DebugSteps:   steps,
		TokenResult:  tokenResult,
		CreatedAt:    t,
	}, nil
}

// DeleteSession 删除会话
func DeleteSession(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}
