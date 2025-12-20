package controllers

import "konsulin-service/internal/app/contracts"

type RoleController struct {
	RoleUsecase contracts.RoleUsecase
}

func NewRoleController(roleUsecase contracts.RoleUsecase) *RoleController {
	return &RoleController{
		RoleUsecase: roleUsecase,
	}
}
