// mautrix-linkedin - A Matrix-LinkedIn puppeting bridge.
// Copyright (C) 2026 Gerardo Rodriguez
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package connector

import (
	"context"
	"fmt"
	"time"

	"maunium.net/go/mautrix/bridgev2/database"
)

const keyLegacyTimestampsFixed database.Key = "linkedin_legacy_timestamps_fixed"

var legacyTimestampCutoff = time.Date(1971, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()

func (l *LinkedInConnector) fixLegacyMigratedTimestamps(ctx context.Context) error {
	if l.Bridge.DB.KV.Get(ctx, keyLegacyTimestampsFixed) == "true" {
		return nil
	}
	log := l.Bridge.Log.With().Str("action", "fix legacy migrated timestamps").Logger()
	for _, table := range []string{"message", "reaction"} {
		res, err := l.Bridge.DB.Exec(ctx, fmt.Sprintf(`
			UPDATE %s
			SET timestamp=CAST(ROUND(timestamp / 1000.0) AS BIGINT) * 1000000
			WHERE bridge_id=$1 AND timestamp > 0 AND timestamp < $2
		`, table), l.Bridge.ID, legacyTimestampCutoff)
		if err != nil {
			return fmt.Errorf("failed to fix %s timestamps: %w", table, err)
		}
		affected, _ := res.RowsAffected()
		log.Info().Str("table", table).Int64("rows", affected).Msg("Fixed legacy migrated timestamps")
	}
	l.Bridge.DB.KV.Set(ctx, keyLegacyTimestampsFixed, "true")
	return nil
}
