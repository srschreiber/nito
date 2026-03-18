create table if not exists migration_version (
    version_num INTEGER PRIMARY KEY,
    applied_at TIMESTAMPTZ DEFAULT now()
);

create table if not exists users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR NOT NULL UNIQUE,
    public_key VARCHAR, -- for encrypting room_keys for users and storing them safely in the database
    updated_at TIMESTAMPTZ DEFAULT now(),
    created_at TIMESTAMPTZ DEFAULT now()
);

create table if not exists permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR NOT NULL UNIQUE, -- e.g. 'rotate_room_keys', 'manage_room_members', etc.
    description VARCHAR
);

create table if not exists role_permissions (
    role VARCHAR NOT NULL, -- e.g. 'admin', 'member', etc.
    permission_id UUID REFERENCES permissions(id) ON DELETE CASCADE,
    updated_at TIMESTAMPTZ DEFAULT now(),
    created_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (role, permission_id)
);

create table if not exists rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR NOT NULL,
    created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ DEFAULT now(),
    created_at TIMESTAMPTZ DEFAULT now()
);

create table if not exists room_members (
    room_id UUID REFERENCES rooms(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    invited_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    joined_at TIMESTAMPTZ, -- null means invitation is pending, set when user accepts
    updated_at TIMESTAMPTZ DEFAULT now(),
    created_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (room_id, user_id)
);

-- role is scoped to a room (e.g. a user can be admin in one room but member in another)
create table if not exists user_room_roles (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    room_id UUID REFERENCES rooms(id) ON DELETE CASCADE,
    role VARCHAR NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now(),
    created_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (user_id, room_id, role)
);

create table if not exists room_invites (
    room_id UUID REFERENCES rooms(id) ON DELETE CASCADE,
    invited_user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    invited_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ, -- null means no expiration
    updated_at TIMESTAMPTZ DEFAULT now(),
    created_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (room_id, invited_user_id, invited_by_user_id)
);

create table if not exists room_key_versions (
    version_num INTEGER NOT NULL,
    room_id UUID REFERENCES rooms(id) ON DELETE CASCADE,
    generated_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ DEFAULT now(),
    created_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (room_id, version_num)
);

-- each user in the room will have their own encrypted copy of the room key for each version, encrypted with their public key
-- each underlying room key will be rotated by users with the right permissions,
-- which will create a new version in room_key_versions and new rows here for each user in the room
create table if not exists user_room_keys (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    room_id UUID REFERENCES rooms(id) ON DELETE CASCADE,
    room_key_version_num INTEGER NOT NULL,
    encrypted_room_key VARCHAR NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now(),
    created_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (user_id, room_id, room_key_version_num),
    FOREIGN KEY (room_id, room_key_version_num) REFERENCES room_key_versions(room_id, version_num) ON DELETE CASCADE
);
