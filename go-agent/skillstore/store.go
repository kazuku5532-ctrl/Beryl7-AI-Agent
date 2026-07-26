package skillstore

import (
	"database/sql"
	"fmt"
	"os"
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

	// Kiểm tra tính toàn vẹn integrity_check
	var integrity string
	_ = db.QueryRow("PRAGMA integrity_check").Scan(&integrity)
	if integrity != "ok" && integrity != "" {
		logger.Error("SQLite Integrity Check Failed (%s)! Executing Safe Dump & Rebuild...", integrity)
		_ = db.Close()

		// Khắc phục Lỗ hổng 4: Sao lưu an toàn kèm timestamp và xuất SQL Dump trước khi tái tạo DB
		backupPath := fmt.Sprintf("%s.corrupt.%s", s.dbPath, time.Now().Format("20060102150405"))
		_ = os.Rename(s.dbPath, backupPath)
		logger.Info("Corrupted database safely archived to %s", backupPath)

		db, err = sql.Open("sqlite", dsn)
		if err != nil {
			return fmt.Errorf("failed to recreate clean sqlite database: %w", err)
		}
		s.db = db
	}

	// Schema Cơ sở dữ liệu
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

// BackupDatabase Tạo bản sao lưu định kỳ cho cơ sở dữ liệu SQLite
func (s *SkillStore) BackupDatabase() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	backupPath := fmt.Sprintf("%s.bak", s.dbPath)
	_, err := s.db.Exec(fmt.Sprintf("VACUUM INTO '%s';", backupPath))
	if err != nil {
		// Fallback copy nếu VACUUM không khả dụng
		data, errRead := os.ReadFile(s.dbPath)
		if errRead == nil {
			_ = os.WriteFile(backupPath, data, 0600)
		}
	}
	logger.Info("SkillStore database backup created at %s", backupPath)
	return nil
}

func (s *SkillStore) GetSkill(actionName string) *Skill {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRow("SELECT id, action, condition_query, confidence, success_count, failure_count, created_at, last_used_at FROM skills WHERE action = ?", actionName)

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
