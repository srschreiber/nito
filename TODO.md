For tracking what I plan on doing next time I work on this.

- Store message contents in postgres (already created a migration)
- Once above is complete, work on creating a ratchet chain (per user in room) using the message count to decrypt messages
- work on retrieving old messages for a user when they join a room to see previous conversation. 
  - need to join on key table on key version to determine what they should be able to decrypt.
- work on room key rotation, initiated by manual, or specific triggers such as new user leaves or joins

nice to haves:
- add sem version so client can check if it's compatible with the server before making requests, o.w. exit with error to update