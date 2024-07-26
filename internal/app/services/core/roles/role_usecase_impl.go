package roles

type roleUsecase struct {
	RoleRepository RoleRepository
}

func NewRoleUsecase(
	roleMongoRepository RoleRepository,
) RoleUsecase {
	return &roleUsecase{
		RoleRepository: roleMongoRepository,
	}
}
