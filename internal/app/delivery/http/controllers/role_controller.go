package controllers

import "konsulin-service/internal/app/services/core/roles"

type RoleController struct {
	RoleUsecase roles.RoleUsecase
}

func NewRoleController(roleUsecase roles.RoleUsecase) *RoleController {
	return &RoleController{
		RoleUsecase: roleUsecase,
	}
}
