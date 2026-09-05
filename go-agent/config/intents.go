package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Intent struct {
	Name                   string   `json:"name"`
	Description            string   `json:"description,omitempty"`
	StartTime              string   `json:"start_time"` // "HH:MM"
	EndTime                string   `json:"end_time"`   // "HH:MM"
	RAMExhaustionPct       *float64 `json:"ram_exhaustion_pct,omitempty"`
	CPUSpikeLoad           *float64 `json:"cpu_spike_load,omitempty"`
	LatencySpikeMs         *float64 `json:"latency_spike_ms,omitempty"`
	LatencyZScoreThreshold *float64 `json:"latency_zscore_threshold,omitempty"`
}

type IntentConfigFile struct {
	Intents []Intent `json:"intents"`
}

type EffectiveThresholds struct {
	ActiveIntent           string  `json:"active_intent"`
	RAMExhaustionPct       float64 `json:"ram_exhaustion_pct"`
	CPUSpikeLoad           float64 `json:"cpu_spike_load"`
	LatencySpikeMs         float64 `json:"latency_spike_ms"`
	LatencyZScoreThreshold float64 `json:"latency_zscore_threshold"`
}

// LoadIntents reads an intents configuration file with a 64KB upper safety bound.
// If the file does not exist, an empty slice and nil error are returned for safe fallback.
func LoadIntents(filePath string) ([]Intent, error) {
	if strings.TrimSpace(filePath) == "" {
		return []Intent{}, nil
	}

	cleanPath := filepath.Clean(filePath)
	file, err := os.Open(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Intent{}, nil
		}
		return []Intent{}, err
	}
	defer file.Close()

	limitReader := io.LimitReader(file, 64*1024)
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return []Intent{}, err
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return []Intent{}, nil
	}

	// First try parsing as IntentConfigFile { "intents": [...] }
	var cfgFile IntentConfigFile
	if err := json.Unmarshal(data, &cfgFile); err == nil && len(cfgFile.Intents) > 0 {
		return cfgFile.Intents, nil
	}

	// Alternatively support direct JSON slice [ {...}, {...} ]
	var directIntents []Intent
	if err := json.Unmarshal(data, &directIntents); err == nil {
		return directIntents, nil
	}

	var testObj interface{}
	if err := json.Unmarshal(data, &testObj); err != nil {
		return []Intent{}, fmt.Errorf("failed to parse intent config JSON: %w", err)
	}

	return cfgFile.Intents, nil
}

// ParseTimeOfDay parses "HH:MM" into the minute of day (0..1439).
func ParseTimeOfDay(hhmm string) (int, error) {
	hhmm = strings.TrimSpace(hhmm)
	parts := strings.Split(hhmm, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid time format '%s', expected HH:MM", hhmm)
	}

	h, err1 := strconv.Atoi(parts[0])
	if err1 != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("invalid hour in time '%s' (must be 00-23)", hhmm)
	}

	m, err2 := strconv.Atoi(parts[1])
	if err2 != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid minute in time '%s' (must be 00-59)", hhmm)
	}

	return h*60 + m, nil
}

// IsTimeInWindow determines if nowMin is within [startMin, endMin), handling overnight wrap-around.
func IsTimeInWindow(nowMin, startMin, endMin int) bool {
	if startMin <= endMin {
		return nowMin >= startMin && nowMin < endMin
	}
	// Overnight wrap-around (e.g., 23:00 to 06:00)
	return nowMin >= startMin || nowMin < endMin
}

// ResolveEffectiveThresholds matches the active intent for the given time and overrides thresholds.
func ResolveEffectiveThresholds(cfg *Config, intents []Intent, now time.Time) EffectiveThresholds {
	ram := 95.0
	cpu := 1.5
	lat := 100.0
	z := 2.5

	if cfg != nil {
		if cfg.RAMExhaustionPct > 0 {
			ram = cfg.RAMExhaustionPct
		}
		if cfg.CPUSpikeLoad > 0 {
			cpu = cfg.CPUSpikeLoad
		}
		if cfg.LatencySpikeMs > 0 {
			lat = cfg.LatencySpikeMs
		}
		if cfg.LatencyZScoreThreshold > 0 {
			z = cfg.LatencyZScoreThreshold
		}
	}

	effective := EffectiveThresholds{
		ActiveIntent:           "default",
		RAMExhaustionPct:       ram,
		CPUSpikeLoad:           cpu,
		LatencySpikeMs:         lat,
		LatencyZScoreThreshold: z,
	}

	if len(intents) == 0 {
		return effective
	}

	nowMin := now.Hour()*60 + now.Minute()

	for _, intent := range intents {
		startMin, err1 := ParseTimeOfDay(intent.StartTime)
		if err1 != nil {
			continue
		}
		endMin, err2 := ParseTimeOfDay(intent.EndTime)
		if err2 != nil {
			continue
		}

		if IsTimeInWindow(nowMin, startMin, endMin) {
			if strings.TrimSpace(intent.Name) != "" {
				effective.ActiveIntent = strings.TrimSpace(intent.Name)
			}
			if intent.RAMExhaustionPct != nil {
				effective.RAMExhaustionPct = *intent.RAMExhaustionPct
			}
			if intent.CPUSpikeLoad != nil {
				effective.CPUSpikeLoad = *intent.CPUSpikeLoad
			}
			if intent.LatencySpikeMs != nil {
				effective.LatencySpikeMs = *intent.LatencySpikeMs
			}
			if intent.LatencyZScoreThreshold != nil {
				effective.LatencyZScoreThreshold = *intent.LatencyZScoreThreshold
			}
			break
		}
	}

	return effective
}
