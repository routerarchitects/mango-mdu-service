package auth

type Context struct {
	UserID string
	Email  string
	Roles  []string
}
