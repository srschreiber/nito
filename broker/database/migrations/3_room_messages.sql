create table if not exists room_messages (
    room_id UUID REFERENCES rooms(id) ON DELETE CASCADE,
    key_version_num INTEGER NOT NULL,
    sender_message_count INTEGER NOT NULL DEFAULT 0, -- this is given by the client
    sender_user_id UUID NOT NULL, -- no foreign key because if user is deleted, we still want to keep the message
    encrypted_text TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (room_id, sender_user_id, key_version_num, sender_message_count)
);
