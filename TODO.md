For tracking what I plan on doing next time I work on this.

~~work on creating a ratchet chain (per user in room) using the message count to decrypt messages~~
- work on retrieving old messages for a user when they join a room to see previous conversation. 
  - need to join on key table on key version to determine what they should be able to decrypt.

Room Connect
- race condition: when connecting to a room, messages might be sent during that request. we should
  - if a message comes in that is greater than a users previous count + 1, request for the messages that came before to backfill
   
- inviting users should not result in a key assignment, and invites cannot now be async. Instead, you invite with a fast expiration of 3 minutes.
  - If the inviter is offline, fail to join
  - when invited user accepts:
    1. status becomes pending_join
    2. inviter triggers key rotation, snapshot will include pending_join users
    3. once key is assigned to users marked as pending_join, update status to active

- work on room key rotation, initiated by manual, or specific triggers such as new user leaves or joins


# Rotation algorithm
1. room rotation lock acquired
2. snapshot current eligible members
3. generate fresh key, increment key version
4. for each eligible member, wrap new room key to that member
5. persist all keys atomically
6. flip current_key_version to new version
7. mark room idle
8. notify clients that new key is available
9. grace window for old key messages that were in flight before key rotation, where in flight means reached broker but not yet processed
   - once notification is sent to change keys, grace time will start (only 5 seconds)
   - handled by sendRoomMessage

nice to haves:
- add sem version so client can check if it's compatible with the server before making requests, o.w. exit with error to update