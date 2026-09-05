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

	"beryl7-agent/logger"
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

// validateAndSanitizeIntents validates that all threshold overrides are strictly positive (> 0).
// Any override <= 0 is safely discarded with a warning, preserving the rest of the intent.
func validateAndSanitizeIntents(intents []Intent) []Intent {
	for i := range intents {
		name := strings.TrimSpace(intents[i].Name)
		if name == "" {
			name = "unnamed_intent"
		}
		if intents[i].RAMExhaustionPct != nil && *intents[i].RAMExhaustionPct <= 0 {
			logger.Warn("INTENT VALIDATION: Intent [%s] has invalid ram_exhaustion_pct (%.2f <= 0) -> Discarding field override", name, *intents[i].RAMExhaustionPct)
			intents[i].RAMExhaustionPct = nil
		}
		if intents[i].CPUSpikeLoad != nil && *intents[i].CPUSpikeLoad <= 0 {
			logger.Warn("INTENT VALIDATION: Intent [%s] has invalid cpu_spike_load (%.2f <= 0) -> Discarding field override", name, *intents[i].CPUSpikeLoad)
			intents[i].CPUSpikeLoad = nil
		}
		if intents[i].LatencySpikeMs != nil && *intents[i].LatencySpikeMs <= 0 {
			logger.Warn("INTENT VALIDATION: Intent [%s] has invalid latency_spike_ms (%.2f <= 0) -> Discarding field override", name, *intents[i].LatencySpikeMs)
			intents[i].LatencySpikeMs = nil
		}
		if intents[i].LatencyZScoreThreshold != nil && *intents[i].LatencyZScoreThreshold <= 0 {
			logger.Warn("INTENT VALIDATION: Intent [%s] has invalid latency_zscore_threshold (%.2f <= 0) -> Discarding field override", name, *intents[i].LatencyZScoreThreshold)
			intents[i].LatencyZScoreThreshold = nil
		}
	}
	return intents
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
		return validateAndSanitizeIntents(cfgFile.Intents), nil
	}

	// Alternatively support direct JSON slice [ {...}, {...} ]
	var directIntents []Intent
	if err := json.Unmarshal(data, &directIntents); err == nil {
		return validateAndSanitizeIntents(directIntents), nil
	}

	var testObj interface{}
	if err := json.Unmarshal(data, &testObj); err != nil {
		return []Intent{}, fmt.Errorf("failed to parse intent config JSON: %w", err)
	}

	return validateAndSanitizeIntents(cfgFile.Intents), nil
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
				if *intent.RAMExhaustionPct > 0 {
					effective.RAMExhaustionPct = *intent.RAMExhaustionPct
				} else {
					logger.Warn("INTENT VALIDATION: Intent [%s] has invalid ram_exhaustion_pct (%.2f <= 0) -> Keeping base threshold (%.2f)", effective.ActiveIntent, *intent.RAMExhaustionPct, effective.RAMExhaustionPct)
				}
			}
			if intent.CPUSpikeLoad != nil {
				if *intent.CPUSpikeLoad > 0 {
					effective.CPUSpikeLoad = *intent.CPUSpikeLoad
				} else {
					logger.Warn("INTENT VALIDATION: Intent [%s] has invalid cpu_spike_load (%.2f <= 0) -> Keeping base threshold (%.2f)", effective.ActiveIntent, *intent.CPUSpikeLoad, effective.CPUSpikeLoad)
				}
			}
			if intent.LatencySpikeMs != nil {
				if *intent.LatencySpikeMs > 0 {
					effective.LatencySpikeMs = *intent.LatencySpikeMs
				} else {
					logger.Warn("INTENT VALIDATION: Intent [%s] has invalid latency_spike_ms (%.2f <= 0) -> Keeping base threshold (%.2f)", effective.ActiveIntent, *intent.LatencySpikeMs, effective.LatencySpikeMs)
				}
			}
			if intent.LatencyZScoreThreshold != nil {
				if *intent.LatencyZScoreThreshold > 0 {
					effective.LatencyZScoreThreshold = *intent.LatencyZScoreThreshold
				} else {
					logger.Warn("INTENT VALIDATION: Intent [%s] has invalid latency_zscore_threshold (%.2f <= 0) -> Keeping base threshold (%.2f)", effective.ActiveIntent, *intent.LatencyZScoreThreshold, effective.LatencyZScoreThreshold)
				}
			}
			break
		}
	}

	return effective
}
