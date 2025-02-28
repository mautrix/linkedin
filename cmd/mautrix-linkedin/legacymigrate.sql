INSERT INTO "user" (bridge_id, mxid, management_room)
SELECT '', mxid, notice_room FROM user_old;

INSERT INTO user_login (bridge_id, user_mxid, id, remote_name, remote_profile, space_room)
SELECT
    '', -- bridge_id
    mxid, -- user_mxid
    li_member_urn, -- id
    li_member_name, -- remote_name
    '{}', -- remote_profile
    space_mxid -- space_room
FROM user_old;

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
    name_set,
    avatar_set,
    contact_info_set,
    false, -- is_bot
    '["linkedin:' || li_member_urn || '"]', -- identifiers
    '{}' -- metadata
FROM puppet_old;

INSERT INTO portal (
    bridge_id, id, receiver, mxid, other_user_id, name, topic, avatar_id, avatar_hash, avatar_mxc,
    name_set, avatar_set, topic_set, name_is_custom, in_space, room_type, metadata
)
SELECT
    '', -- bridge_id
    'urn:li:msg_conversation:(urn:li:fsd_profile:' || li_receiver_urn || ',' || li_urn_thread || ')', -- id
    CASE WHEN li_is_group_chat=1 THEN '' ELSE li_receiver_urn END, -- receiver
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
    'urn:li:msg_conversation:(urn:li:fsd_profile:' || portal_old.li_receiver_urn || ',' || portal_old.li_urn_thread || ')', -- portal_id
    CASE WHEN NOT li_is_group_chat THEN li_receiver_urn, -- portal_receiver
    false, -- in_space
    false -- preferred
FROM portal_old
JOIN user_old ON user_old.li_member_urn = portal_old.li_receiver_urn;

INSERT INTO message (
    bridge_id, id, part_id, mxid, room_id, room_receiver, sender_id, sender_mxid, timestamp, metadata
)
SELECT
    '', -- bridge_id
    (
        'urn:li:msg_message:(urn:li:fsd_profile:' ||
        li_receiver_urn ||
        ',' ||
        SUBSTR(li_message_urn, INSTR(li_message_urn, ',')) ||
        ')'
    ), -- id
    '', -- part_id
    mxid,
    'urn:li:msg_conversation:(urn:li:fsd_profile:' || li_receiver_urn || ',' || li_thread_urn || ')', -- room_id
    CASE WHEN li_receiver_urn<>0 THEN CAST(li_receiver_urn AS TEXT) ELSE '' END, -- room_receiver
    li_sender_urn, -- sender_id
    '', -- sender_mxid
    timestamp * 1000000, -- timestamp
    '{}' -- metadata
FROM message_old;

INSERT INTO reaction (
    bridge_id, message_id, message_part_id, sender_id, emoji_id, room_id, room_receiver, mxid, timestamp, emoji, metadata
)
SELECT
    '', -- bridge_id
    (
        'urn:li:msg_message:(urn:li:fsd_profile:' ||
        li_receiver_urn ||
        ',' ||
        SUBSTR(li_message_urn, INSTR(li_message_urn, ',')) ||
        ')'
    ), -- message_id
    '', -- message_part_id
    li_sender_urn, -- sender_id
    '', -- emoji_id
    mx_room, -- room_id
    li_receiver_urn, -- room_receiver
    mxid,
    (
        SELECT (timestamp * 1000000) + 1
        FROM message_old
        WHERE message_old.li_message_urn=reacton_old.li_message_urn
            AND index=0
            AND message_old.li_receiver_urn=reaction_old.li_receiver_urn
    ), -- timestamp
    reaction, -- emoji
    '{}' -- metadata
FROM reaction_old;

DROP TABLE message_old;
DROP TABLE portal_old;
DROP TABLE puppet_old;
DROP TABLE reaction_old;
DROP TABLE user_old;
DROP TABLE user_portal_old;
