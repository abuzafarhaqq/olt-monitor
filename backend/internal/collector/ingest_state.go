package collector

import (
	"context"
	"fmt"
	"time"
)

// DeviceState is what a run learned about the OLT itself, as opposed to its ONUs.
//
// The inventory's "last collection" column reads this table. Nothing wrote it,
// so a device could be collected every hour and still show the timestamp from the
// day it was seeded — which is exactly what happened.
type DeviceState struct {
	Reachable        bool
	Firmware         string
	Hardware         string
	ClockTrusted     bool
	CollectionStatus string // successful | partial | failed
}

func (in Ingest) RecordDeviceState(ctx context.Context, oltID string, state DeviceState, at time.Time) error {
	quality := "untrusted"
	if state.ClockTrusted {
		quality = "trusted"
	}

	_, err := in.Pool.Exec(ctx, `
		INSERT INTO olt_state (olt_id, reachable, firmware, hardware_version, device_time,
		                       clock_quality, observed_at, collection_status)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULL, $5::timestamp_quality, $6, $7::collection_status)
		ON CONFLICT (olt_id) DO UPDATE
		   SET reachable = EXCLUDED.reachable,
		       firmware = COALESCE(EXCLUDED.firmware, olt_state.firmware),
		       hardware_version = COALESCE(EXCLUDED.hardware_version, olt_state.hardware_version),
		       clock_quality = EXCLUDED.clock_quality,
		       observed_at = EXCLUDED.observed_at,
		       collection_status = EXCLUDED.collection_status`,
		oltID, state.Reachable, state.Firmware, state.Hardware, quality, at, state.CollectionStatus)
	if err != nil {
		return fmt.Errorf("record device state: %w", err)
	}
	return nil
}
