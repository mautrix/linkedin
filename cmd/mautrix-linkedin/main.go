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
	"net/http"

	"maunium.net/go/mautrix/bridgev2/bridgeconfig"
	"maunium.net/go/mautrix/bridgev2/matrix/mxmain"

	"go.mau.fi/util/dbutil"

	"go.mau.fi/mautrix-linkedin/pkg/connector"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

// Information to find out exactly which commit the bridge was built from.
// These are filled at build time with the -X linker flag.
var (
	Tag       = "unknown"
	Commit    = "unknown"
	BuildTime = "unknown"
)

var m = mxmain.BridgeMain{
	Name:        "mautrix-linkedin",
	URL:         "https://github.com/mautrix/linkedin",
	Description: "A Matrix-LinkedIn puppeting bridge.",
	Version:     "0.6.0",
	Connector:   &connector.LinkedInConnector{},
}

func main() {
	bridgeconfig.HackyMigrateLegacyNetworkConfig = migrateLegacyConfig
	m.PostStart = func() {
		if m.Matrix.Provisioning != nil {
			m.Matrix.Provisioning.Router.HandleFunc("/v1/api/login", legacyProvLogin).Methods(http.MethodPost)
			m.Matrix.Provisioning.Router.HandleFunc("/v1/api/logout", legacyProvLogout).Methods(http.MethodPost)
		}
	}
	m.PostInit = func() {
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
	m.InitVersion(Tag, Commit, BuildTime)
	m.Run()
}
