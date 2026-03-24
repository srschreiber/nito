package keys

type Key []byte
type RoomKeyChain struct {
	userChain    map[string]Key
	baseKey      Key
	chainCounter map[string]int
}

func NewRoomKeyChain(baseKey Key) *RoomKeyChain {
	return &RoomKeyChain{
		userChain:    map[string]Key{},
		baseKey:      baseKey,
		chainCounter: map[string]int{},
	}
}

// GetUserKey derives the user Key using a ratchet mechanism to ensure forward secrecy.
func (kc *RoomKeyChain) GetUserKey(userId string, messageCount int) (Key, error) {
	if _, ok := kc.userChain[userId]; !ok {
		kc.userChain[userId] = kc.baseKey
		kc.chainCounter[userId] = 0
	}

	if messageCount == kc.chainCounter[userId] {
		return kc.userChain[userId], nil
	}

	for kc.chainCounter[userId] < messageCount {
		c := kc.chainCounter[userId]
		next := GenerateMessageEncryptionKey(kc.userChain[userId], FormatHMACInput(userId, &c))
		kc.userChain[userId] = next
		kc.chainCounter[userId]++
	}

	return kc.userChain[userId], nil
}
