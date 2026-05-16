package models

// Registration is both the POST /auth/register JSON body and the
// input to [UsersService.Create]. Bounds mirror the openapi spec:
// username 1-64, password 8-256.
type Registration struct {
	Username string `json:"username" binding:"required,min=1,max=64"`
	Password string `json:"password" binding:"required,min=8,max=256"`
}
