package users

type userUsecase struct {
	UserRepository UserRepository
	UserFhirClient UserFhirClient
}

func NewUserUsecase(
	userMongoRepository UserRepository,
	userFhirClient UserFhirClient,
) UserUsecase {
	return &userUsecase{
		UserRepository: userMongoRepository,
		UserFhirClient: userFhirClient,
	}
}
