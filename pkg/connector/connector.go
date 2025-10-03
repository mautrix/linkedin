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

package connector

import (
	"context"

	"github.com/google/uuid"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type LinkedInConnector struct {
	Bridge *bridgev2.Bridge
	Config Config
}

var _ bridgev2.NetworkConnector = (*LinkedInConnector)(nil)
var _ bridgev2.TransactionIDGeneratingNetwork = (*LinkedInConnector)(nil)

func (l *LinkedInConnector) Init(bridge *bridgev2.Bridge) {
	l.Bridge = bridge
}

func (l *LinkedInConnector) Start(ctx context.Context) error {
	return nil
}

func (l *LinkedInConnector) LoadUserLogin(ctx context.Context, login *bridgev2.UserLogin) error {
	login.Client = NewLinkedInClient(ctx, l, login)
	return nil
}

func (lc *LinkedInConnector) GetName() bridgev2.BridgeName {
	return bridgev2.BridgeName{
		DisplayName:      "LinkedIn",
		NetworkURL:       "https://linkedin.com",
		NetworkIcon:      "mxc://maunium.net/CqzBEHjrLsfdqixWZgNHMlRT",
		NetworkID:        "linkedin",
		BeeperBridgeType: "linkedin",
		DefaultPort:      29341,
	}
}

func (lc *LinkedInConnector) GenerateTransactionID(userID id.UserID, roomID id.RoomID, eventType event.Type) networkid.RawTransactionID {
	return networkid.RawTransactionID(uuid.NewString())
}
