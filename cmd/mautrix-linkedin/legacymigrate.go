// mautrix-linkedin - A Matrix-LinkedIn puppeting bridge.
// Copyright (C) 2025 Sumner Evans
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

package main

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"unicode"

	"github.com/rs/zerolog"
	up "go.mau.fi/util/configupgrade"
	"go.mau.fi/util/dbutil"
	"maunium.net/go/mautrix/appservice"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/mautrix-linkedin/pkg/connector"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

const legacyMigrateRenameTables = `
ALTER TABLE cookie RENAME TO cookie_old;
ALTER TABLE message RENAME TO message_old;
ALTER TABLE portal RENAME TO portal_old;
ALTER TABLE puppet RENAME TO puppet_old;
ALTER TABLE reaction RENAME TO reaction_old;
ALTER TABLE "user" RENAME TO user_old;
ALTER TABLE user_portal RENAME TO user_portal_old;
`

//go:embed legacymigrate.sql
var legacyMigrateCopyData string

func migrateLegacyConfig(helper up.Helper) {
	helper.Set(up.Str, "mautrix.bridge.e2ee", "encryption", "pickle_key")
}

func MigrateLegacyDB() {
	log := m.Log.With().Str("component", "database_migrator").Logger()
	ctx := log.WithContext(context.TODO())

	// The old bridge does not have a database_owner table, so use that to
	// detect if the migration has already happened.
	exists, err := m.DB.TableExists(ctx, "database_owner")
	if err != nil {
		log.Err(err).Msg("Failed to check if database_owner table exists")
		return
	} else if exists {
		log.Debug().Msg("Database owner table exists, assuming database is already migrated")
		return
	}
	exists, err = m.DB.TableExists(ctx, "version")
	if err != nil {
		log.Err(err).Msg("Failed to check if version table exists")
		return
	} else if !exists {
		log.Debug().Msg("Version table does not exist, assuming database is empty")
		return
	}

	expectedVersion := 10
	var dbVersion int
	err = m.DB.QueryRow(ctx, "SELECT version FROM version").Scan(&dbVersion)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get database version")
	} else if dbVersion < expectedVersion {
		log.Fatal().
			Int("expected_version", expectedVersion).
			Int("version", dbVersion).
			Msg("Unsupported database version. Please upgrade to beeper/linkedin v0.5.4 or higher before upgrading to v0.6.0.")
		return
	} else if dbVersion > expectedVersion {
		log.Fatal().
			Int("expected_version", expectedVersion).
			Int("version", dbVersion).
			Msg("Unsupported database version (higher than expected)")
		return
	}
	log.Info().Msg("Detected legacy database, migrating...")
	err = m.DB.DoTxn(ctx, nil, func(ctx context.Context) error {
		if err := m.LegacyMigrateSimple(legacyMigrateRenameTables, legacyMigrateCopyData, 16)(ctx); err != nil {
			return err
		}
		rows, err := m.DB.Query(ctx, "SELECT mxid, name, value FROM cookie_old")
		if err != nil {
			return err
		}
		cookies := map[string]*linkedingo.StringCookieJar{}
		for rows.Next() {
			var mxid, name, value string
			if err := rows.Scan(&mxid, &name, &value); err != nil {
				return err
			}
			if _, ok := cookies[mxid]; !ok {
				cookies[mxid] = linkedingo.NewEmptyStringCookieJar()
			}
			cookies[mxid].AddCookie(&http.Cookie{Name: name, Value: value})
		}
		for mxid, jar := range cookies {
			metadata := connector.UserLoginMetadata{Cookies: jar}
			if _, err := m.DB.Exec(ctx, "UPDATE user_login SET metadata = $1 WHERE user_mxid = $2", dbutil.JSON{Data: metadata}, mxid); err != nil {
				return err
			}
		}
		_, err = m.DB.Exec(ctx, "DROP TABLE cookie_old;")
		return err
	})
	if err != nil {
		m.LogDBUpgradeErrorAndExit("main", err, "Failed to migrate legacy database")
	} else {
		log.Info().Msg("Successfully migrated legacy database")
	}
}

func PostMigratePortal(ctx context.Context, portal *bridgev2.Portal) error {
	oldGhostRe := m.Config.MakeUserIDRegex("(.+)")
	// If this is a DM, we need to invite the bot and make it the admin.
	if portal.OtherUserID != "" {
		localpart := m.Config.AppService.FormatUsername(string(portal.OtherUserID))
		intent := m.Matrix.AS.Intent(id.NewUserID(localpart, m.Matrix.AS.HomeserverDomain))

		// Figure out which of the accounts is actually the admin of the room.
		pls, err := intent.PowerLevels(ctx, portal.MXID)
		if err != nil {
			return fmt.Errorf("failed to get power levels in room: %w", err)
		}
		for userID, level := range pls.Users {
			if level == 100 {
				intent = m.Matrix.AS.Intent(userID)
				break
			}
		}

		// Ensure that the bridge bot is joined to the room.
		err = m.Matrix.Bot.EnsureJoined(ctx, portal.MXID, appservice.EnsureJoinedParams{
			BotOverride: intent.Client,
		})
		if err != nil {
			return fmt.Errorf("failed to ensure bot is joined to DM (%s / %s): %w", portal.ID, portal.MXID, err)
		}

		// Make the bot the admin of the room, and demote the ghost user.
		userLevel := pls.GetUserLevel(intent.UserID)
		pls.EnsureUserLevel(intent.UserID, pls.UsersDefault)
		pls.EnsureUserLevel(m.Matrix.Bot.UserID, userLevel)
		_, err = intent.SetPowerLevels(ctx, portal.MXID, pls)
		if err != nil {
			return fmt.Errorf("failed to set power levels in room (%s / %s): %w", portal.ID, portal.MXID, err)
		}
	} else if portal.RoomType == database.RoomTypeDM {
		zerolog.Ctx(ctx).Warn().
			Str("portal_id", string(portal.ID)).
			Msg("DM portal has no other user ID")
		return nil
	}

	members, err := m.Matrix.GetMembers(ctx, portal.MXID)
	if err != nil {
		return err
	}
	for userID, member := range members {
		if member.Membership != event.MembershipJoin {
			continue
		}
		if userID == m.Matrix.Bot.UserID {
			continue
		}

		// Detect legacy user IDs which are not all lowercase
		var hasUpper bool
		for _, c := range userID.Localpart() {
			hasUpper = hasUpper || unicode.IsUpper(c)
		}
		if !hasUpper {
			continue
		}

		// Remove the legacy user from the room
		_, err = m.Matrix.AS.Intent(userID).LeaveRoom(ctx, portal.MXID)
		if err != nil {
			return fmt.Errorf("failed to leave room with legacy user ID %s: %w", userID, err)
		}

		// Join the new ghost to the room
		match := oldGhostRe.FindStringSubmatch(string(userID))
		if match == nil {
			return fmt.Errorf("failed to parse ghost MXID %s", userID)
		}
		networkUserID := networkid.UserID(match[1])
		ghost, err := m.Bridge.GetGhostByID(ctx, networkUserID)
		if err != nil {
			return fmt.Errorf("failed to get ghost for %s: %w", networkUserID, err)
		}
		err = ghost.Intent.EnsureJoined(ctx, portal.MXID)
		if err != nil {
			return fmt.Errorf("failed to ensure ghost is joined to portal (%s / %s): %w", portal.ID, portal.MXID, err)
		}
	}

	if portal.RoomType == database.RoomTypeDM {
		ghost, err := m.Bridge.GetGhostByID(ctx, portal.OtherUserID)
		if err != nil {
			return err
		}
		portal.UpdateInfoFromGhost(ctx, ghost)
	}
	return nil
}
