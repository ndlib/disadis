package disseminator

import (
	"net/http"
)

type Auth interface {
	Check(r *http.Request, id string, isThumb bool) bool
}

type alwaysYes struct{}

func NewPermitEverything() Auth {
	return &alwaysYes{}
}

func (sy *alwaysYes) Check(r *http.Request, id string, isThumb bool) bool {
	return true
}
