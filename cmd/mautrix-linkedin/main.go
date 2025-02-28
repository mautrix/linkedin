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
	"go.mau.fi/util/exerrors"

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
		m.CheckLegacyDB(
			10,
			"v0.5.4",
			"v0.6.0",
			m.LegacyMigrateSimple(legacyMigrateRenameTables, legacyMigrateCopyData, 16),
			true,
		)

		ctx := context.TODO()
		rows := exerrors.Must(m.DB.Query(ctx, "SELECT mxid, name, value FROM cookie"))
		cookies := map[string]*linkedingo.StringCookieJar{}
		for rows.Next() {
			var mxid, name, value string
			exerrors.PanicIfNotNil(rows.Scan(&mxid, &name, &value))
			if _, ok := cookies[mxid]; !ok {
				cookies[mxid] = linkedingo.NewEmptyStringCookieJar()
			}
			cookies[mxid].AddCookie(&http.Cookie{Name: name, Value: value})
		}
		for mxid, jar := range cookies {
			metadata := connector.UserLoginMetadata{Cookies: jar}
			exerrors.Must(m.DB.Exec(ctx, "UPDATE user_login SET metadata = $1 WHERE mxid = $2", dbutil.JSON{Data: metadata}, mxid))
		}
		exerrors.Must(m.DB.Exec(ctx, `
			DROP TABLE cookie_old;
			DROP TABLE http_header_old;
		`))
	}
	m.InitVersion(Tag, Commit, BuildTime)
	m.Run()
}
