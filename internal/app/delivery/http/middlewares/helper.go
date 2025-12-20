package middlewares

import "errors"

func parseUser(data map[string]interface{}) (User, error) {
	var user User

	if sub, ok := data["sub"].(string); ok {
		user.ID = sub
	} else {
		return user, errors.New("sub not found or invalid type")
	}

	if stRoleRaw, ok := data["st-role"].(map[string]interface{}); ok {
		if vRaw, ok := stRoleRaw["v"]; ok {
			if roles, ok := vRaw.([]string); ok {
				user.Roles = roles
			} else if roleInterfaces, ok := vRaw.([]interface{}); ok {
				for _, role := range roleInterfaces {
					if roleStr, ok := role.(string); ok {
						user.Roles = append(user.Roles, roleStr)
					}
				}
			} else {
				return user, errors.New("st-role.v is not a slice")
			}
		} else {
			return user, errors.New("st-role.v key missing")
		}
	} else {
		return user, errors.New("st-role is not a map")
	}

	return user, nil
}
