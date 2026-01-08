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
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"

	"go.mau.fi/mautrix-linkedin/pkg/linkedingo"
)

func (l *LinkedInClient) makePortalKey(conv linkedingo.Conversation) (key networkid.PortalKey) {
	key.ID = networkid.PortalID(conv.EntityURN.String())
	if !conv.GroupChat || l.main.Bridge.Config.SplitPortals {
		key.Receiver = l.userLogin.ID
	}
	return key
}

func (l *LinkedInClient) makeSender(participant linkedingo.MessagingParticipant) (sender bridgev2.EventSender) {
	id := participant.EntityURN.ID()
	sender.IsFromMe = id == string(l.userID)
	sender.Sender = networkid.UserID(id)
	sender.SenderLogin = networkid.UserLoginID(id)
	return
}

func MakeUserLoginID(userID string) networkid.UserLoginID {
	return networkid.UserLoginID(userID)
}

func ParseUserLoginID(userID networkid.UserLoginID) string {
	return string(userID)
}

type MediaInfo struct {
	UserID    networkid.UserLoginID
	MessageID networkid.MessageID
	PartID    networkid.PartID
}

func appendBytes(ret []byte, data []byte) []byte {
	ret = binary.AppendUvarint(ret, uint64(len(data)))
	ret, err := binary.Append(ret, binary.BigEndian, data)
	if err != nil {
		panic(err)
	}
	return ret
}

func readBytes(buf *bytes.Reader) (string, error) {
	size, err := binary.ReadUvarint(buf)
	if err != nil {
		return "", err
	}
	bs := make([]byte, size)
	_, err = io.ReadFull(buf, bs)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func parseMessageID(messageID networkid.MessageID) (string, string) {
	urn := linkedingo.NewURN(messageID)
	idLen := len(urn.ID())
	parts := strings.Split(urn.ID()[1:idLen-1], ",")
	profileID, msgID := parts[0], parts[1]
	return linkedingo.NewURN(profileID).ID(), msgID
}

func makeMessageID(senderID string, msgID string) networkid.MessageID {
	id := fmt.Sprintf("urn:li:msg_message:(urn:li:fsd_profile:%s,%s)", senderID, msgID)
	return networkid.MessageID(id)
}

func MakeMediaID(userID networkid.UserLoginID, messageID networkid.MessageID, partID networkid.PartID) networkid.MediaID {
	mediaID := []byte{1}
	_, msgID := parseMessageID(messageID)
	mediaID = appendBytes(mediaID, []byte(userID))
	mediaID = appendBytes(mediaID, []byte(msgID))
	mediaID = appendBytes(mediaID, []byte(partID))
	return mediaID
}

func ParseMediaID(mediaID networkid.MediaID) (*MediaInfo, error) {
	buf := bytes.NewReader(mediaID)
	version := make([]byte, 1)
	_, err := io.ReadFull(buf, version)
	if err != nil {
		return nil, err
	}
	if version[0] != byte(1) {
		return nil, fmt.Errorf("unknown mediaID version: %v", version)
	}

	mediaInfo := &MediaInfo{}
	userID, err := readBytes(buf)
	if err != nil {
		return nil, err
	}
	mediaInfo.UserID = MakeUserLoginID(userID)

	msgID, err := readBytes(buf)
	if err != nil {
		return nil, err
	}
	mediaInfo.MessageID = makeMessageID(userID, msgID)

	str, err := readBytes(buf)
	if err != nil {
		return nil, err
	}
	mediaInfo.PartID = networkid.PartID(str)

	return mediaInfo, nil
}
