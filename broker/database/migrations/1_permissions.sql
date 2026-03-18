insert into permissions (name, description) values
    ('rotate_keys',         'Rotate the room encryption keys'),
    ('invite_members',      'Invite new members to the room'),
    ('kick_members',        'Remove members from the room'),
    ('change_member_roles', 'Change the role of a room member'),
    ('manage_room_settings','Update room name and other settings'),
    ('view_audit_log',      'View the room audit log')
;

insert into role_permissions (role, permission_id)
select 'admin', id from permissions
;
