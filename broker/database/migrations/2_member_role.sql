-- members can invite others and view the audit log (key rotation and moderation are admin-only)
insert into role_permissions (role, permission_id)
select 'member', id from permissions
where name in ('invite_members', 'view_audit_log')
;
