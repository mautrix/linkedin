# Features & roadmap

* Matrix → LinkedIn
  * [ ] Message content
    * [x] Text
    * [x] Media
      * [x] Files
      * [x] Images
      * [x] Videos (sent as files)
      * [x] GIFs
      * [ ] Voice Messages
      * [ ] ~~Stickers~~ (unsupported)
    * [ ] ~~Formatting~~ (LinkedIn does not support rich formatting)
    * [ ] Replies
    * [x] Mentions
    * [x] Emotes
  * [x] Message edits
  * [x] Message redactions
  * [ ] Message reactions
  * [ ] Presence
  * [ ] Typing notifications
  * [ ] Read receipts
  * [ ] Power level
  * [ ] Membership actions
    * [ ] Invite
    * [ ] Kick
    * [ ] Leave
  * [ ] Room metadata changes
    * [ ] Name
    * [ ] Avatar
    * [ ] Per-room user nick
* LinkedIn → Matrix
  * [x] Message content
    * [x] Text
    * [x] Media
      * [x] Files
      * [x] Images
      * [x] GIFs
      * [x] Videos
      * [x] Voice Messages
    * [x] Mentions
  * [x] Message edits
  * [x] Message delete
  * [ ] Message reactions
  * [ ] Message history
  * [x] Real-time messages
  * [ ] ~~Presence~~ (impossible for now, see https://github.com/mautrix/go/issues/295)
  * [x] Typing notifications
  * [x] Read receipts
  * [ ] Admin status
  * [ ] Membership actions
    * [x] Add member
    * [x] Remove member
    * [ ] Leave
  * [x] Chat metadata changes
    * [x] Title
    * [ ] ~Avatar~ (group chats don't have avatars in LinkedIn)
  * [x] Initial chat metadata
  * [x] User metadata
    * [x] Name
    * [x] Avatar
* Misc
  * [ ] Multi-user support
  * [ ] Shared group chat portals
  * [ ] Automatic portal creation
    * [ ] At startup
    * [ ] When added to chat
    * [x] When receiving message
  * [ ] Private chat creation by inviting Matrix puppet of LinkedIn user to new room
  * [ ] Option to use own Matrix account for messages sent from other LinkedIn clients (relay mode)
  * [ ] Split portal support
