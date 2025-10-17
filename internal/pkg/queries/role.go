package queries

const (
	GetAllRoles = `
		SELECT id, name, created_at, updated_at, deleted_at
		FROM roles
		WHERE deleted_at IS NULL
	`

	GetAllRolesWithPermissions = `
		SELECT 
			r.id AS role_id,
			r.name AS role_name,
			COALESCE(json_agg(
				json_build_object(
					'resource', rp.resource,
					'actions', rp.actions
				)
			) FILTER (WHERE rp.id IS NOT NULL), '[]') AS permissions
		FROM roles r
		LEFT JOIN role_permissions rp ON r.id = rp.role_id
		WHERE r.deleted_at IS NULL
		GROUP BY r.id, r.name
		ORDER BY r.id;
	`

	GetRoleWithPermissionsByID = `
		SELECT 
			r.id AS role_id,
			r.name AS role_name,
			json_agg(
				json_build_object(
					'resource', rp.resource,
					'actions', rp.actions
				)
			) AS permissions
		FROM roles r
		LEFT JOIN role_permissions rp ON r.id = rp.role_id
		WHERE r.id = $1 AND r.deleted_at IS NULL
		GROUP BY r.id, r.name
	`

	GetRoleWithPermissionsByName = `
		SELECT 
			r.id AS role_id,
			r.name AS role_name,
			json_agg(
				json_build_object(
					'resource', rp.resource,
					'actions', rp.actions
				)
			) AS permissions
		FROM roles r
		LEFT JOIN role_permissions rp ON r.id = rp.role_id
		WHERE r.name = $1 AND r.deleted_at IS NULL
		GROUP BY r.id, r.name
	`

	GetRoleByID = `
		SELECT id, name, created_at, updated_at, deleted_at
		FROM roles
		WHERE id = $1 AND deleted_at IS NULL
	`

	GetRoleByName = `
		SELECT id, name, created_at, updated_at, deleted_at
		FROM roles
		WHERE name = $1 AND deleted_at IS NULL
	`

	InsertRole = `
		INSERT INTO roles (name)
		VALUES ($1)
		RETURNING id
	`

	UpdateRole = `
		UPDATE roles
		SET name = $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`

	DeleteRole = `
		UPDATE roles
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`
)
