package roles

import "konsulin-service/internal/app/contracts"

type roleUsecase struct {
	RoleRepository contracts.RoleRepository
}

func NewRoleUsecase(
	roleMongoRepository contracts.RoleRepository,
) contracts.RoleUsecase {
	return &roleUsecase{
		RoleRepository: roleMongoRepository,
	}
}
