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

package linkedinfmt_test

import (
	"context"
	"embed"
	"encoding/json"
	"io"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/mautrix-linkedin/pkg/connector/linkedinfmt"
	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

//go:embed attributedtext/*
var attributedtextFS embed.FS

var linkedinFmtParams = linkedinfmt.FormatParams{
	GetMXIDByURN: func(ctx context.Context, entityURN linkedingo.URN) (id.UserID, error) {
		return id.UserID(entityURN.ID()), nil
	},
}

func TestParse(t *testing.T) {
	entries, err := attributedtextFS.ReadDir("attributedtext")
	require.NoError(t, err)

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_msg.json") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			f, err := attributedtextFS.Open(path.Join("attributedtext", entry.Name()))
			require.NoError(t, err)
			expectedMsgJSONFile, err := attributedtextFS.Open(path.Join("attributedtext", strings.TrimSuffix(entry.Name(), ".json")+"_msg.json"))
			require.NoError(t, err)
			expectedMsgJSON, err := io.ReadAll(expectedMsgJSONFile)
			require.NoError(t, err)

			var attributedText linkedingo.AttributedText
			err = json.NewDecoder(f).Decode(&attributedText)
			assert.NoError(t, err)

			content, err := linkedinfmt.Parse(context.TODO(), attributedText.Text, attributedText.Attributes, linkedinFmtParams)
			assert.NoError(t, err)

			marshalled, err := json.Marshal(content)
			assert.NoError(t, err)

			assert.JSONEq(t, string(expectedMsgJSON), string(marshalled))
		})
	}

}
