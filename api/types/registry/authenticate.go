package registry

// AuthenticateOKBody authenticate o k body
type AuthenticateOKBody struct {

	// An opaque token used to authenticate a user after a successful login
	// Required: true
	PGPKey string `json:"pgp_key"`

	// The status of the authentication
	// Required: true
	Status string `json:"status"`
}
