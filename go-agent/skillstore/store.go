package skillstore

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"beryl7-agent/logger"
	_ "modernc.org/sqlite"
)

type Skill struct {
	ID           string    `json:"id"`
	Action       string    `json:"action"`
	Condition    string    `json:"condition"`
	Confidence   float64   `json:"confidence"`
	SuccessCount int       `json:"success_count"`
	FailureCount int       `json:"failure_count"`
	CreatedAt    time.Time `json:"created_at"`
	LastUsedAt   time.Time `json:"last_used_at"`
}

type SkillStore struct {
	mu            sync.RWMutex
	db            *sql.DB
	dbPath        string
	lastPruneTime time.Time
	maxSkills     int
}

func New(dbPath string) (*SkillStore, error) {
	s := &SkillStore{
		dbPath:        dbPath,
		maxSkills:     1000,
		lastPruneTime: time.Now(),
	}

	if err := s.OpenAndInit(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *SkillStore) OpenAndInit() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dsn := fmt.Sprintf("file:%s?cache=shared&mode=rwc", s.dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	s.db = db

	if _, err := db.Exec(`
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = NORMAL;
		PRAGMA temp_store = MEMORY;
		PRAGMA busy_timeout = 5000;
		PRAGMA wal_autocheckpoint = 1000;
	`); err != nil {
		logger.Warn("Failed to set SQLite pragmas: %v", err)
	}

	var integrity string
	_ = db.QueryRow("PRAGMA integrity_check").Scan(&integrity)
	if integrity != "ok" && integrity != "" {
		logger.Error("CRITICAL SQLITE INTEGRITY FAILURE (%s)! Initiating emergency salvage procedure...", integrity)
		_ = db.Close()

		// 1. Sao lưu file bị hỏng
		backupPath := fmt.Sprintf("%s.corrupt.%s", s.dbPath, time.Now().Format("20060102150405"))
		_ = os.Rename(s.dbPath, backupPath)
		logger.Error("Corrupted DB safely archived to %s for offline forensic salvage", backupPath)

		// 2. Thử xuất SQL Dump khôi phục trực tiếp qua CLI `sqlite3 .dump`
		dumpPath := filepath.Clean(backupPath + ".sql")
		dumpCmd := exec.Command("sqlite3", filepath.Clean(backupPath), ".dump") // #nosec G204
		if dumpOut, dumpErr := dumpCmd.Output(); dumpErr == nil && len(dumpOut) > 0 {
			_ = os.WriteFile(dumpPath, dumpOut, 0600) // #nosec G703
			logger.Info("Exported SQLite SQL Dump salvage file to %s (%d bytes)", dumpPath, len(dumpOut))
		}

		// 3. Khôi phục từ bản sao lưu snapshot (.bak) gần nhất nếu có
		bakPath := filepath.Clean(fmt.Sprintf("%s.bak", s.dbPath))
		if bakData, errBak := os.ReadFile(bakPath); errBak == nil && len(bakData) > 0 { // #nosec G304 G703
			if writeErr := os.WriteFile(filepath.Clean(s.dbPath), bakData, 0600); writeErr == nil { // #nosec G703
				logger.Info("SUCCESSFULLY SALVAGED database from recent backup snapshot %s!", bakPath)
			}
		}

		db, err = sql.Open("sqlite", dsn)
		if err != nil {
			return fmt.Errorf("failed to reopen sqlite database: %w", err)
		}
		s.db = db
	}

	schema := `
	CREATE TABLE IF NOT EXISTS skills (
		id TEXT PRIMARY KEY,
		action TEXT NOT NULL,
		condition_query TEXT NOT NULL,
		confidence REAL NOT NULL DEFAULT 0.5,
		success_count INTEGER NOT NULL DEFAULT 0,
		failure_count INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		last_used_at INTEGER NOT NULL
	);
	PRAGMA user_version = 1;
	`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create skillstore schema: %w", err)
	}

	return nil
}

func (s *SkillStore) BackupDatabase() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	backupPath := fmt.Sprintf("%s.bak", s.dbPath)
	cleanDbPath := filepath.Clean(s.dbPath)
	cleanBackupPath := filepath.Clean(backupPath)
	_, err := s.db.Exec(fmt.Sprintf("VACUUM INTO '%s';", cleanBackupPath))
	if err != nil {
		data, errRead := os.ReadFile(cleanDbPath) // #nosec G304
		if errRead == nil {
			_ = os.WriteFile(cleanBackupPath, data, 0600) // #nosec G703
		}
	}
	logger.Info("SkillStore database backup created at %s", cleanBackupPath)
	return nil
}

func (s *SkillStore) GetSkill(condition, actionName string) *Skill {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var row *sql.Row
	if condition != "" {
		row = s.db.QueryRow("SELECT id, action, condition_query, confidence, success_count, failure_count, created_at, last_used_at FROM skills WHERE condition_query = ? AND action = ?", condition, actionName)
	} else {
		row = s.db.QueryRow("SELECT id, action, condition_query, confidence, success_count, failure_count, created_at, last_used_at FROM skills WHERE action = ?", actionName)
	}

	var sk Skill
	var created, lastUsed int64
	if err := row.Scan(&sk.ID, &sk.Action, &sk.Condition, &sk.Confidence, &sk.SuccessCount, &sk.FailureCount, &created, &lastUsed); err != nil {
		return nil
	}
	sk.CreatedAt = time.Unix(created, 0)
	sk.LastUsedAt = time.Unix(lastUsed, 0)
	return &sk
}

func (s *SkillStore) SaveOrUpdateSkill(sk *Skill, isSuccess bool, alpha float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	if sk.CreatedAt.IsZero() {
		sk.CreatedAt = time.Now()
	}
	sk.LastUsedAt = time.Now()

	target := 0.0
	if isSuccess {
		target = 1.0
		sk.SuccessCount++
	} else {
		sk.FailureCount++
	}

	sk.Confidence = sk.Confidence + alpha*(target-sk.Confidence)

	query := `
	INSERT INTO skills (id, action, condition_query, confidence, success_count, failure_count, created_at, last_used_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		confidence = excluded.confidence,
		success_count = excluded.success_count,
		failure_count = excluded.failure_count,
		last_used_at = excluded.last_used_at;
	`

	_, err := s.db.Exec(query, sk.ID, sk.Action, sk.Condition, sk.Confidence, sk.SuccessCount, sk.FailureCount, sk.CreatedAt.Unix(), now)
	if err != nil {
		return fmt.Errorf("failed to save skill: %w", err)
	}

	if isSuccess {
		_, _ = s.db.Exec("PRAGMA wal_checkpoint(PASSIVE);")
	}

	return nil
}

func (s *SkillStore) PruneSkillsPeriodic() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM skills").Scan(&count)

	if time.Since(s.lastPruneTime) < 24*time.Hour && count < s.maxSkills {
		return nil
	}

	s.lastPruneTime = time.Now()

	thresholdTime := time.Now().AddDate(0, 0, -30).Unix()
	createdThreshold := time.Now().AddDate(0, 0, -90).Unix()

	query := `DELETE FROM skills WHERE confidence < 0.05 AND last_used_at < ? AND created_at < ?`
	res, err := s.db.Exec(query, thresholdTime, createdThreshold)
	if err == nil {
		rows, _ := res.RowsAffected()
		if rows > 0 {
			logger.Info("Pruned %d low-confidence skills from SkillStore", rows)
		}
	}

	if count >= s.maxSkills {
		_, _ = s.db.Exec("DELETE FROM skills WHERE id IN (SELECT id FROM skills ORDER BY confidence ASC LIMIT 100)")
		logger.Warn("SkillStore reached 1000 skills capacity limit! Auto-pruned 100 lowest confidence skills.")
	}

	return nil
}

func (s *SkillStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		_ = s.db.Close()
	}
	return nil
}
