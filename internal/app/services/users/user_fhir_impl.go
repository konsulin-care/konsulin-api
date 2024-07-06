package users

type userFhirClient struct {
	BaseUrl string
}

func NewUserFhirClient(userFhirBaseUrl string) UserFhirClient {
	return &userFhirClient{
		BaseUrl: userFhirBaseUrl,
	}
}
