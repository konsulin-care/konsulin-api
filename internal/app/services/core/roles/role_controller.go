package roles

type RoleController struct {
	RoleUsecase RoleUsecase
}

func NewRoleController(roleUsecase RoleUsecase) *RoleController {
	return &RoleController{
		RoleUsecase: roleUsecase,
	}
}
