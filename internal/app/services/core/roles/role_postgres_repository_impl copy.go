package roles

import (
	"context"
	"database/sql"
	"encoding/json"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/queries"
	"sync"

	"go.uber.org/zap"
)

type rolePostgresRepository struct {
	DB  *sql.DB
	Log *zap.Logger
}

var (
	rolePostgresRepositoryInstance contracts.RoleRepository
	onceRolePostgresRepository     sync.Once
)

func NewRolePostgresRepository(db *sql.DB, logger *zap.Logger) contracts.RoleRepository {
	onceRolePostgresRepository.Do(func() {
		instance := &rolePostgresRepository{
			DB:  db,
			Log: logger,
		}
		rolePostgresRepositoryInstance = instance
	})
	return rolePostgresRepositoryInstance
}
func (repo *rolePostgresRepository) FindAll(ctx context.Context) ([]models.Role, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info("rolePostgresRepository.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	query := queries.GetAllRolesWithPermissions
	rows, err := repo.DB.QueryContext(ctx, query)
	if err != nil {
		repo.Log.Error("rolePostgresRepository.FindAll error executing query",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var role models.Role
		var permissionsJSON string

		if err := rows.Scan(&role.ID, &role.Name, &permissionsJSON); err != nil {
			repo.Log.Error("rolePostgresRepository.FindAll error scanning row",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrPostgresDBFindData(err)
		}

		if err := json.Unmarshal([]byte(permissionsJSON), &role.Permissions); err != nil {
			repo.Log.Error("rolePostgresRepository.FindAll error unmarshaling permissions JSON",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrPostgresDBFindData(err)
		}

		roles = append(roles, role)
	}

	if err := rows.Err(); err != nil {
		repo.Log.Error("rolePostgresRepository.FindAll rows iteration error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	repo.Log.Info("rolePostgresRepository.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingRoleCountKey, len(roles)),
	)
	return roles, nil
}

func (repo *rolePostgresRepository) CreateRole(ctx context.Context, entityRole *models.Role) (roleID string, err error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info("rolePostgresRepository.CreateRole called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRoleNameKey, entityRole.Name),
	)

	query := queries.InsertRole
	var id string
	err = repo.DB.QueryRowContext(ctx, query, entityRole.Name).Scan(&id)
	if err != nil {
		repo.Log.Error("rolePostgresRepository.CreateRole error executing insert",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return "", exceptions.ErrPostgresDBInsertData(err)
	}

	repo.Log.Info("rolePostgresRepository.CreateRole succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRoleIDKey, id),
	)
	return id, nil
}

func (repo *rolePostgresRepository) FindByName(ctx context.Context, roleName string) (*models.Role, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info("rolePostgresRepository.FindByName called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRoleNameKey, roleName),
	)

	query := queries.GetRoleWithPermissionsByName
	var role models.Role
	var permissionsJSON string

	err := repo.DB.QueryRowContext(ctx, query, roleName).Scan(&role.ID, &role.Name, &permissionsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			repo.Log.Warn("rolePostgresRepository.FindByName no rows found",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String(constvars.LoggingRoleNameKey, roleName),
			)
			return nil, nil
		}
		repo.Log.Error("rolePostgresRepository.FindByName error executing query",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRoleNameKey, roleName),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	if err := json.Unmarshal([]byte(permissionsJSON), &role.Permissions); err != nil {
		repo.Log.Error("rolePostgresRepository.FindByName error unmarshaling permissions JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	repo.Log.Info("rolePostgresRepository.FindByName succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRoleIDKey, role.ID),
	)
	return &role, nil
}

func (repo *rolePostgresRepository) FindRoleByID(ctx context.Context, roleID string) (*models.Role, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info("rolePostgresRepository.FindRoleByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRoleIDKey, roleID),
	)

	query := queries.GetRoleWithPermissionsByID
	var role models.Role
	var permissionsJSON string

	err := repo.DB.QueryRowContext(ctx, query, roleID).Scan(&role.ID, &role.Name, &permissionsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			repo.Log.Warn("rolePostgresRepository.FindRoleByID no rows found",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String(constvars.LoggingRoleIDKey, roleID),
			)
			return nil, nil
		}
		repo.Log.Error("rolePostgresRepository.FindRoleByID error executing query",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRoleIDKey, roleID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	if err := json.Unmarshal([]byte(permissionsJSON), &role.Permissions); err != nil {
		repo.Log.Error("rolePostgresRepository.FindRoleByID error unmarshaling permissions JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	repo.Log.Info("rolePostgresRepository.FindRoleByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRoleIDKey, role.ID),
	)
	return &role, nil
}

func (repo *rolePostgresRepository) UpdateRole(ctx context.Context, roleID string, updateData map[string]interface{}) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	repo.Log.Info("rolePostgresRepository.UpdateRole called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRoleIDKey, roleID),
		zap.Any("update_data", updateData),
	)

	query := queries.UpdateRole
	_, err := repo.DB.ExecContext(ctx, query, roleID, updateData)
	if err != nil {
		repo.Log.Error("rolePostgresRepository.UpdateRole error executing update",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingRoleIDKey, roleID),
			zap.Error(err),
		)
		return exceptions.ErrPostgresDBUpdateData(err)
	}

	repo.Log.Info("rolePostgresRepository.UpdateRole succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingRoleIDKey, roleID),
	)
	return nil
}
