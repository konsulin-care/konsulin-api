package middlewares

import "errors"

func parseUser(data map[string]interface{}) (User, error) {
	sub, err := extractSub(data)
	if err != nil {
		return User{}, err
	}

	roles, err := extractRolesPayload(data)
	if err != nil {
		return User{}, err
	}

	return User{ID: sub, Roles: roles}, nil
}

// extractSub retrieves the "sub" field from a claims map.
func extractSub(data map[string]interface{}) (string, error) {
	sub, ok := data["sub"].(string)
	if !ok {
		return "", errors.New("sub not found or invalid type")
	}
	return sub, nil
}

// extractRolesPayload retrieves the roles from a SuperTokens "st-role.v" claims structure.
func extractRolesPayload(data map[string]interface{}) ([]string, error) {
	stRoleRaw, ok := data["st-role"].(map[string]interface{})
	if !ok {
		return nil, errors.New("st-role is not a map")
	}

	vRaw, ok := stRoleRaw["v"]
	if !ok {
		return nil, errors.New("st-role.v key missing")
	}

	if roles, ok := vRaw.([]string); ok {
		return roles, nil
	}

	if roleInterfaces, ok := vRaw.([]interface{}); ok {
		var roles []string
		for _, role := range roleInterfaces {
			if roleStr, ok := role.(string); ok {
				roles = append(roles, roleStr)
			}
		}
		return roles, nil
	}

	return nil, errors.New("st-role.v is not a slice")
}
