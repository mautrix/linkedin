INSERT INTO "user" (bridge_id, mxid, management_room)
SELECT '', mxid, notice_room FROM user_old;

INSERT INTO user_login (bridge_id, user_mxid, id, remote_name, remote_profile, space_room, metadata)
SELECT
    '', -- bridge_id
    mxid, -- user_mxid
    li_member_urn, -- id
    li_member_urn, -- remote_name
    '{}', -- remote_profile
    space_mxid, -- space_room
    '{}' -- metadata
FROM user_old
WHERE li_member_urn<>'';

INSERT INTO ghost (
    bridge_id, id, name, avatar_id, avatar_hash, avatar_mxc,
    name_set, avatar_set, contact_info_set, is_bot, identifiers, metadata
)
SELECT
    '', -- bridge_id
    li_member_urn, -- id
    COALESCE(name, ''), -- name
    COALESCE(photo_id, ''), -- avatar_id
    '', -- avatar_hash
    COALESCE(photo_mxc, ''), -- avatar_mxc
    false, -- name_set (need to set it on the new case-insensitive ghost mxid)
    false, -- avatar_set (need to set it on the new case-insensitive ghost mxid)
    false, -- contact_info_set (need to set it on the new case-insensitive ghost mxid)
    false, -- is_bot
    -- only: postgres
    jsonb_build_array
    -- only: sqlite (line commented)
--  json_array
    (
        'linkedin:' || li_member_urn
    ), -- identifiers
    '{}' -- metadata
FROM puppet_old;

INSERT INTO portal (
    bridge_id, id, receiver, mxid, other_user_id, name, topic, avatar_id, avatar_hash, avatar_mxc,
    name_set, avatar_set, topic_set, name_is_custom, in_space, room_type, metadata
)
SELECT
    '', -- bridge_id
    'urn:li:msg_conversation:(urn:li:fsd_profile:' || li_receiver_urn || ',' || li_thread_urn || ')', -- id
    CASE WHEN NOT li_is_group_chat THEN li_receiver_urn ELSE '' END, -- receiver
    mxid, -- mxid
    CASE WHEN NOT li_is_group_chat THEN li_other_user_urn END, -- other_user_id
    COALESCE(name, ''), -- name
    COALESCE(topic, ''), -- topic
    COALESCE(photo_id, ''), -- avatar_id
    '', -- avatar_hash
    COALESCE(avatar_url, ''), -- avatar_mxc
    name_set, -- name_set
    avatar_set, -- avatar_set
    topic_set, -- topic_set
    li_is_group_chat, -- name_is_custom
    false, -- in_space
    CASE WHEN li_is_group_chat THEN 'dm' ELSE 'group_dm' END, -- room_type
    '{}' -- metadata
FROM portal_old;

INSERT INTO user_portal (bridge_id, user_mxid, login_id, portal_id, portal_receiver, in_space, preferred)
SELECT
    '', -- bridge_id
    user_old.mxid, -- user_mxid
    user_old.li_member_urn, -- login_id
    'urn:li:msg_conversation:(urn:li:fsd_profile:' || portal_old.li_receiver_urn || ',' || portal_old.li_thread_urn || ')', -- portal_id
    CASE WHEN NOT li_is_group_chat THEN li_receiver_urn ELSE '' END, -- portal_receiver
    false, -- in_space
    false -- preferred
FROM portal_old
JOIN user_old ON user_old.li_member_urn = portal_old.li_receiver_urn;

INSERT INTO message (
    bridge_id, id, part_id, mxid, room_id, room_receiver, sender_id, sender_mxid, timestamp, edit_count, metadata
)
SELECT
    '', -- bridge_id
    (
        'urn:li:msg_message:(urn:li:fsd_profile:' ||
        li_receiver_urn ||
        SUBSTR(li_message_urn,
            -- only: postgres
            POSITION(',' IN li_message_urn)
            -- only: sqlite (line commented)
--          INSTR(li_message_urn, ',')
        ) ||
        ')'
    ), -- id
    '', -- part_id
    mxid,
    'urn:li:msg_conversation:(urn:li:fsd_profile:' || li_receiver_urn || ',' || li_thread_urn || ')', -- room_id
    (
        SELECT CASE WHEN NOT li_is_group_chat THEN li_receiver_urn ELSE '' END
        FROM portal_old
        WHERE li_thread_urn=message_old.li_thread_urn
    ), -- room_receiver
    li_sender_urn, -- sender_id
    '', -- sender_mxid
    timestamp * 1000000, -- timestamp
    0, -- edit_count
    '{}' -- metadata
FROM message_old
WHERE true
ON CONFLICT DO NOTHING;

INSERT INTO reaction (
    bridge_id, message_id, message_part_id, sender_id, emoji_id, room_id, room_receiver, mxid, timestamp, emoji, metadata
)
SELECT
    '', -- bridge_id
    (
        'urn:li:msg_message:(urn:li:fsd_profile:' ||
        li_receiver_urn ||
        SUBSTR(li_message_urn,
            -- only: postgres
            POSITION(',' IN li_message_urn)
            -- only: sqlite (line commented)
--          INSTR(li_message_urn, ',')
        ) ||
        ')'
    ), -- message_id
    '', -- message_part_id
    li_sender_urn, -- sender_id
    reaction, -- emoji_id
    'urn:li:msg_conversation:(urn:li:fsd_profile:' || li_receiver_urn || ',' || SUBSTR(li_message_urn, 0,
        -- only: postgres
        POSITION(',' IN li_message_urn)
        -- only: sqlite (line commented)
--      INSTR(li_message_urn, ',')
    ) || ')', -- room_id
    (
        SELECT CASE WHEN NOT li_is_group_chat THEN li_receiver_urn ELSE '' END
        FROM portal_old
        WHERE li_thread_urn=SUBSTR(li_message_urn, 0,
        -- only: postgres
        POSITION(',' IN li_message_urn)
        -- only: sqlite (line commented)
--      INSTR(li_message_urn, ',')
    )
    ), -- room_receiver
    mxid,
    (
        SELECT (timestamp * 1000000) + 1
        FROM message_old
        WHERE message_old.li_message_urn=reaction_old.li_message_urn
            AND "index"=0
            AND message_old.li_receiver_urn=reaction_old.li_receiver_urn
    ), -- timestamp
    reaction, -- emoji
    '{}' -- metadata
FROM reaction_old WHERE EXISTS(
    SELECT 1
    FROM message_old
    WHERE message_old.li_message_urn=reaction_old.li_message_urn
        AND "index"=0
        AND message_old.li_receiver_urn=reaction_old.li_receiver_urn
)
ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS database_owner (
	key   INTEGER PRIMARY KEY DEFAULT 0,
	owner TEXT NOT NULL
);
INSERT INTO database_owner (key, owner) VALUES (0, "megabridge/mautrix-linkedin");

-- Python -> Go mx_ table migration
ALTER TABLE mx_room_state DROP COLUMN is_encrypted;
ALTER TABLE mx_room_state RENAME COLUMN has_full_member_list TO members_fetched;
UPDATE mx_room_state SET members_fetched=false WHERE members_fetched IS NULL;

-- only: postgres until "end only"
ALTER TABLE mx_room_state ALTER COLUMN power_levels TYPE jsonb USING power_levels::jsonb;
ALTER TABLE mx_room_state ALTER COLUMN encryption TYPE jsonb USING encryption::jsonb;
ALTER TABLE mx_room_state ALTER COLUMN members_fetched SET DEFAULT false;
ALTER TABLE mx_room_state ALTER COLUMN members_fetched SET NOT NULL;
-- end only postgres

ALTER TABLE mx_user_profile ADD COLUMN name_skeleton bytea;
CREATE INDEX mx_user_profile_membership_idx ON mx_user_profile (room_id, membership);
CREATE INDEX mx_user_profile_name_skeleton_idx ON mx_user_profile (room_id, name_skeleton);

UPDATE mx_user_profile SET displayname='' WHERE displayname IS NULL;
UPDATE mx_user_profile SET avatar_url='' WHERE avatar_url IS NULL;

CREATE TABLE mx_registrations (
    user_id TEXT PRIMARY KEY
);

UPDATE mx_version SET version=7;

DROP TABLE message_old;
DROP TABLE puppet_old;
DROP TABLE reaction_old;
DROP TABLE user_portal_old;
DROP TABLE user_old;
DROP TABLE portal_old;
