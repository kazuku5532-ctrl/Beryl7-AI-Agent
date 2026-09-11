package main

import (
	"context"

	"beryl7-agent/logger"
	"beryl7-agent/notifier"
	"beryl7-agent/skillstore"
)

// runTelemetryMaintenance executes the 24-hour scheduled telemetry maintenance cycle:
//  1. Prunes stale SkillStore entries (periodic compaction).
//  2. Prunes telemetry history records older than retentionDays.
//  3. Logs current telemetry history footprint.
//  4. Fires the one-shot 14-day readiness notification via Telegram when the
//     dataset has accumulated at least 14 days of continuous telemetry data.
//
// This function is called exclusively from the case <-pruneTicker.C branch of the
// main telemetry select loop. It must not be called from any other goroutine directly.
func runTelemetryMaintenance(
	ctx context.Context,
	store *skillstore.SkillStore,
	retentionDays int,
	tgNotifier *notifier.TelegramNotifier,
) {
	// Step 1: Prune stale SkillStore entries.
	if pruneErr := store.PruneSkillsPeriodic(); pruneErr != nil {
		logger.Error("Scheduled SkillStore pruning failed: %v", pruneErr)
	}

	// Step 2: Prune telemetry history records beyond the retention window.
	if prunedCount, errPrune := store.PruneTelemetryHistory(ctx, retentionDays); errPrune != nil {
		logger.Warn("Failed to prune telemetry history: %v", errPrune)
	} else if prunedCount > 0 {
		logger.Info("PRUNED TELEMETRY HISTORY: Removed %d records older than %d days", prunedCount, retentionDays)
	}

	// Step 3: Log current telemetry history footprint and conditionally fire
	// the one-shot 14-day data readiness Telegram notification.
	if stats, errStats := store.GetTelemetryHistoryStats(ctx); errStats == nil {
		logger.Info("TELEMETRY HISTORY FOOTPRINT: %d total records, estimated %d bytes in DB",
			stats.TotalRecords, stats.EstimatedBytes)

		// One-shot 14-day telemetry data readiness check for Predictive Analysis (Phase 2b).
		if isNotified, errLatch := store.IsMilestoneLatchSet("telemetry_14d_readiness_notified"); errLatch == nil && !isNotified {
			if stats.TotalRecords > 0 && (stats.NewestUnix-stats.OldestUnix) >= 14*86400 {
				if tgNotifier != nil {
					go func(oldest, newest, total int64) {
						if errSend := tgNotifier.SendTelemetryReadinessAlert(context.Background(), oldest, newest, total); errSend != nil {
							logger.Warn("TELEMETRY READINESS: Failed to send Telegram alert: %v (will retry on next maintenance cycle)", errSend)
						} else {
							if errSet := store.SetMilestoneLatch("telemetry_14d_readiness_notified"); errSet != nil {
								logger.Warn("TELEMETRY READINESS: Failed to persist milestone latch: %v", errSet)
							} else {
								logger.Info("TELEMETRY READINESS: 14-day telemetry data readiness alert successfully dispatched and latched.")
							}
						}
					}(stats.OldestUnix, stats.NewestUnix, stats.TotalRecords)
				}
			}
		}
	}
}
