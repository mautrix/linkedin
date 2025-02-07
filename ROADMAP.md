# Features & roadmap

* Matrix → LinkedIn
  * [ ] Message content
    * [ ] Text
    * [ ] Media
      * [ ] Files
      * [ ] Images
      * [ ] Videos
      * [ ] GIFs
      * [ ] Voice Messages
      * [ ] ~~Stickers~~ (unsupported)
    * [ ] ~~Formatting~~ (LinkedIn does not support rich formatting)
    * [ ] Replies
    * [ ] Mentions
    * [ ] Emotes
  * [ ] Message edits
  * [ ] Message redactions
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
  * [ ] Message content
    * [x] Text
    * [ ] Media
      * [ ] Files
      * [ ] Images
      * [ ] GIFs
      * [ ] Voice Messages
    * [ ] Mentions
  * [x] Message edits
  * [x] Message delete
  * [ ] Message reactions
  * [ ] Message history
  * [ ] Real-time messages
  * [ ] ~~Presence~~ (impossible for now, see https://github.com/mautrix/go/issues/295)
  * [ ] Typing notifications
  * [ ] Read receipts
  * [ ] Admin status
  * [ ] Membership actions
    * [ ] Add member
    * [ ] Remove member
    * [ ] Leave
  * [ ] Chat metadata changes
    * [ ] Title
    * [ ] Avatar
  * [ ] Initial chat metadata
  * [ ] User metadata
    * [ ] Name
    * [ ] Avatar
* Misc
  * [ ] Multi-user support
  * [ ] Shared group chat portals
  * [ ] Automatic portal creation
    * [ ] At startup
    * [ ] When added to chat
    * [ ] When receiving message (not supported)
  * [ ] Private chat creation by inviting Matrix puppet of LinkedIn user to new room
  * [ ] Option to use own Matrix account for messages sent from other LinkedIn clients (relay mode)
  * [ ] Split portal support
