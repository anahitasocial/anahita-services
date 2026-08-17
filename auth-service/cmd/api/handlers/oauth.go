package handlers

type oauthHandler struct {
	*Env
}

func newOAuthHandler(env *Env) *oauthHandler {
	return &oauthHandler{
		env,
	}
}
