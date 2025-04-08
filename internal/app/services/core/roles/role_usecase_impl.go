package roles

import (
	"context"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"sync"

	"github.com/supertokens/supertokens-golang/recipe/userroles"
	"go.uber.org/zap"
)

type roleUsecase struct {
	RoleRepository contracts.RoleRepository
	Log            *zap.Logger
}

var (
	roleUsecaseInstance contracts.RoleUsecase
	onceRoleUsecase     sync.Once
)

func NewRoleUsecase(
	roleRepository contracts.RoleRepository,
	logger *zap.Logger,
) contracts.RoleUsecase {
	onceRoleUsecase.Do(func() {
		instance := &roleUsecase{
			RoleRepository: roleRepository,
			Log:            logger,
		}

		ctx := context.Background()
		instance.initializeData(ctx)
		roleUsecaseInstance = instance
	})

	return roleUsecaseInstance
}

func (uc *roleUsecase) initializeData(ctx context.Context) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("roleUsecase.initializeData called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	resp, err := userroles.CreateNewRoleOrAddPermissions(constvars.KonsulinRolePatient, []string{
		"read", "write",
	}, nil)

	if err != nil {
		uc.Log.Error("roleUsecase.initializeData error CreateNewRoleOrAddPermissions with supertokens",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
	}

	if !resp.OK.CreatedNewRole {
		uc.Log.Info("roleUsecase.initializeData role PATIENT already exists",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
	}

	resp, err = userroles.CreateNewRoleOrAddPermissions(constvars.KonsulinRoleClinician, []string{
		"read", "write",
	}, nil)

	if err != nil {
		uc.Log.Error("roleUsecase.initializeData error CreateNewRoleOrAddPermissions with supertokens",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
	}

	if !resp.OK.CreatedNewRole {
		uc.Log.Info("roleUsecase.initializeData role CLINICIAN already exists",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
	}

	uc.Log.Info("roleUsecase.initializeData completed",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
}
