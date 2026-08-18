package skillstore

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"beryl7-agent/logger"
	_ "modernc.org/sqlite"
)

var ErrStoreClosed = fmt.Errorf("skillstore: database connection is closed")

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
	closed        bool
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

func NewHybrid(ramPath, flashPath string) (*SkillStore, error) {
	cleanRAM := filepath.Clean(ramPath)
	cleanFlash := filepath.Clean(flashPath)

	_ = os.MkdirAll(filepath.Dir(cleanRAM), 0750)

	// If working RAM DB doesn't exist, restore from persistent Flash DB
	if _, errRAM := os.Stat(cleanRAM); os.IsNotExist(errRAM) {
		if flashData, errFlash := os.ReadFile(cleanFlash); errFlash == nil && len(flashData) > 0 { // #nosec G304, G703
			if errWrite := os.WriteFile(cleanRAM, flashData, 0600); errWrite == nil {
				logger.Info("HYBRID STORE: Restored SkillStore from NAND Flash (%s) to RAM tmpfs working DB (%s)", cleanFlash, cleanRAM)
			}
		}
	}

	return New(cleanRAM)
}

func (s *SkillStore) OpenAndInit() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// [Fix 4] _busy_timeout=5000: SQLite queues for up to 5s on write-lock contention (VACUUM INTO, goroutine writes) instead of immediately returning SQLITE_BUSY
	dsn := fmt.Sprintf("file:%s?cache=shared&mode=rwc&_busy_timeout=5000", s.dbPath)
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
	row := db.QueryRow("PRAGMA integrity_check")
	err = row.Scan(&integrity)
	if err != nil {
		logger.Error("CRITICAL: Database integrity check query failed (connection issue): %v", err)
	} else if integrity != "ok" {
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
		if bakData, errBak := os.ReadFile(bakPath); errBak == nil && len(bakData) > 0 { // #nosec G304, G703
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
	CREATE TABLE IF NOT EXISTS q_table (
		state TEXT,
		action TEXT,
		q_value REAL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (state, action)
	);
	PRAGMA user_version = 1;
	`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create skillstore schema: %w", err)
	}

	// Seed Prior Q-Table values for cold-start acceleration without overwriting learned values
	seedQTable := `
	INSERT OR IGNORE INTO q_table (state, action, q_value) VALUES
		('WAN_DROP', 'restart_wan_interface', 0.5),
		('MEMORY_EXHAUSTION', 'purge_memory_cache', 0.6),
		('WIFI_FAILURE', 'optimize_wifi_channel', 0.5),
		('LATENCY_SPIKE', 'restart_interface', 0.5),
		('BUFFERBLOAT_SPIKE', 'enable_cake_sqm', 0.6),
		('REPEATER_SIGNAL_WEAK', 'scale_tx_power_down', 0.5),
		('REPEATER_CHANNEL_CONGESTED', 'align_channels', 0.5);
	`
	_, _ = db.Exec(seedQTable)

	return nil
}

func (s *SkillStore) BackupDatabase() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	backupPath := fmt.Sprintf("%s.bak", s.dbPath)
	cleanDbPath := filepath.Clean(s.dbPath)
	cleanBackupPath := filepath.Clean(backupPath)

	data, errRead := os.ReadFile(cleanDbPath) // #nosec G304
	if errRead != nil {
		logger.Error("SkillStore database backup read failed: %v", errRead)
		return errRead
	}

	errWrite := os.WriteFile(cleanBackupPath, data, 0600) // #nosec G703
	if errWrite != nil {
		logger.Error("SkillStore database backup write failed: %v", errWrite)
		return errWrite
	}

	logger.Info("SkillStore database backup created securely at %s", cleanBackupPath)
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

func (s *SkillStore) GetBestSkillForAnomaly(condition string) *Skill {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed || s.db == nil {
		return nil
	}

	row := s.db.QueryRow("SELECT id, action, condition_query, confidence, success_count, failure_count, created_at, last_used_at FROM skills WHERE condition_query = ? ORDER BY confidence DESC, success_count DESC LIMIT 1", condition)

	var sk Skill
	var created, lastUsed int64
	if err := row.Scan(&sk.ID, &sk.Action, &sk.Condition, &sk.Confidence, &sk.SuccessCount, &sk.FailureCount, &created, &lastUsed); err != nil {
		return nil
	}

	// Harmonize SkillStore with Q-Table: Verify Q-Value for the skill's action
	var qVal float64
	qErr := s.db.QueryRow("SELECT q_value FROM q_table WHERE state = ? AND action = ?", condition, sk.Action).Scan(&qVal)
	if qErr == nil {
		if qVal < 0.0 {
			// Skill has negative Q-Learning feedback: do not recommend obsolete skill!
			logger.Warn("SKILLSTORE HARMONIZATION: Skill [%s] for Anomaly [%s] rejected due to negative Q-Value (%.2f)", sk.Action, condition, qVal)
			return nil
		}
		origConf := sk.Confidence
		// Weight confidence by Q-Value to keep sources of truth aligned
		sk.Confidence = sk.Confidence * math.Max(0.1, qVal)
		if sk.Confidence < origConf {
			logger.Info("SKILLSTORE HARMONIZATION: Weighted Skill [%s] confidence from %.2f -> %.2f based on Q-Value (%.2f)", sk.Action, origConf, sk.Confidence, qVal)
		}
	}

	sk.CreatedAt = time.Unix(created, 0)
	sk.LastUsedAt = time.Unix(lastUsed, 0)
	return &sk
}

func (s *SkillStore) SaveOrUpdateSkill(sk *Skill, isSuccess bool, alpha float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed || s.db == nil {
		return ErrStoreClosed
	}

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

func TranslateSkillInterface(command, fromVersion, toVersion string) string {
	if fromVersion == "4.9.0" && (toVersion == "5.0" || strings.HasPrefix(toVersion, "5.")) {
		command = strings.ReplaceAll(command, "eth0", "wan0")
	}
	return command
}

func (s *SkillStore) FilterCompatibleSkills(fwVersion string) []*Skill {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT id, action, condition_query, confidence, success_count, failure_count, created_at, last_used_at FROM skills WHERE confidence >= 0.50 ORDER BY confidence DESC")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var compatible []*Skill
	for rows.Next() {
		var sk Skill
		var created, lastUsed int64
		if err := rows.Scan(&sk.ID, &sk.Action, &sk.Condition, &sk.Confidence, &sk.SuccessCount, &sk.FailureCount, &created, &lastUsed); err == nil {
			sk.CreatedAt = time.Unix(created, 0)
			sk.LastUsedAt = time.Unix(lastUsed, 0)
			compatible = append(compatible, &sk)
		}
	}
	return compatible
}

func (s *SkillStore) GetTopSkillsSummary(limit int) string {
	return s.GetTopSkillsSummaryForAnomaly("", limit)
}

func (s *SkillStore) GetTopSkillsSummaryForAnomaly(anomalyType string, limit int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 5
	}

	var rows *sql.Rows
	var err error

	if anomalyType != "" {
		rows, err = s.db.Query("SELECT condition_query, action, confidence, success_count FROM skills WHERE condition_query = ? AND confidence >= 0.70 ORDER BY success_count DESC, confidence DESC LIMIT ?", anomalyType, limit)
	} else {
		rows, err = s.db.Query("SELECT condition_query, action, confidence, success_count FROM skills WHERE confidence >= 0.70 ORDER BY success_count DESC, confidence DESC LIMIT ?", limit)
	}

	if err != nil {
		return ""
	}
	defer rows.Close()

	var sb strings.Builder
	for rows.Next() {
		var cond, act string
		var conf float64
		var succ int
		if err := rows.Scan(&cond, &act, &conf, &succ); err == nil {
			sb.WriteString(fmt.Sprintf("- When '%s' -> Action '%s' (Confidence: %.2f, Successes: %d)\n", cond, act, conf, succ))
		}
	}
	return sb.String()
}

func (s *SkillStore) FlushToPersistent(persistentPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.db == nil {
		return ErrStoreClosed
	}

	cleanPersistent := filepath.Clean(persistentPath)
	if err := os.MkdirAll(filepath.Dir(cleanPersistent), 0750); err != nil {
		return fmt.Errorf("failed to create directory for persistent DB: %w", err)
	}

	tmpBackup := fmt.Sprintf("%s.tmp", cleanPersistent)
	_ = os.Remove(tmpBackup)

	// SQLite Native Atomic Snapshot: VACUUM INTO creates a transactionally consistent, uncorrupted snapshot file directly managed by SQLite
	vacuumSQL := fmt.Sprintf("VACUUM INTO '%s'", strings.ReplaceAll(tmpBackup, "'", "''")) // #nosec G201
	if _, err := s.db.Exec(vacuumSQL); err != nil {
		return fmt.Errorf("SQLite VACUUM INTO atomic backup failed: %w", err)
	}

	// Atomic rename onto persistent Flash destination
	if err := os.Rename(tmpBackup, cleanPersistent); err != nil {
		_ = os.Remove(tmpBackup)
		return fmt.Errorf("failed to atomic rename persistent database: %w", err)
	}

	logger.Info("HYBRID STORE: Successfully executed atomic SQLite VACUUM INTO snapshot to persistent Flash (%s)", cleanPersistent)
	return nil
}

const learningRate = 0.2 // Learning rate Alpha

// UpdateQValue executes single-step Q-learning Bellman update: Q(s,a) = Q(s,a) + Alpha * (Reward - Q(s,a))
// Bound Q-value to [-0.8, 1.0] using SQLite MAX/MIN native SQL functions to prevent negative penalty lockouts.
func (s *SkillStore) UpdateQValue(state string, action string, reward float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.db == nil {
		return ErrStoreClosed
	}

	_, err := s.db.Exec(`
		INSERT INTO q_table (state, action, q_value) 
		VALUES (?, ?, ?)
		ON CONFLICT(state, action) DO UPDATE SET 
		q_value = MAX(-0.8, MIN(1.0, q_value + ? * (? - q_value))),
		updated_at = CURRENT_TIMESTAMP`,
		state, action, reward, learningRate, reward)
	return err
}

// RecommendBestAction queries Q-Table for action with highest Q-value for the current anomaly state.
// On Cold Start (no learned Q-table entries exist for state), gracefully returns defaultAction with 0.0 Q-value without error.
func (s *SkillStore) RecommendBestAction(state string, defaultAction string) (string, float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed || s.db == nil {
		return "", 0, ErrStoreClosed
	}

	var bestAction string
	var qValue float64
	err := s.db.QueryRow(`
		SELECT action, q_value FROM q_table 
		WHERE state = ? 
		ORDER BY q_value DESC, updated_at DESC LIMIT 1`, state).Scan(&bestAction, &qValue)

	if err != nil {
		// Cold Start Handling: Return defaultAction with 0.0 Q-value when no learned history exists
		return defaultAction, 0.0, nil
	}
	return bestAction, qValue, nil
}

func (s *SkillStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.db != nil {
		_ = s.db.Close()
	}
	return nil
}
