package auth

import (
	"encoding/xml"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dbrower/disadis/fedora"
)

func NewHydraAuth(fedoraPath, namespace string) *HydraAuth {
	return &HydraAuth{
		fedora: fedora.NewRemote(fedoraPath, namespace),
	}
}

// HydraAuth will validate requests against Hydra rights metadata stored
// in some fedora instance. It can either be used as an http.Handler, wrapping
// a target handler, or independently in your own handler.
//
// The RequestUser is used to determine the current user given a request.
// It may make HTTP calls or perform database lookups to resolve things,
// ultimately returning a username and a list of groups the user belongs to.
// The zero value for the User is the anonymous user who belongs to no groups.
//
// To use it as a wrapping handler, give it a Handler to wrap and an optional
// IdExtractor to return an object identifier given a URL. The default extractor
// takes the first path component in the URL.
// This interface may need to be generalized to be a
//	func(*http.Request) string
//
// To just use the checking in your own handler call Check() directly.
type HydraAuth struct {
	CurrentUser RequestUser // determines the current user
	// Extract a Fedora object identifier from a URL
	// If nil then the first component in the path is taken to be the identifier
	IdExtractor func(string) string
	Handler     http.Handler // handler to pass authorized requests to
	fedora      fedora.Fedora       // interface to Fedora
}

func (ha *HydraAuth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if ha.Handler == nil {
		http.Error(w, "404 Not found", http.StatusNotFound)
		return
	}
	var id string
	if ha.IdExtractor == nil {
		ha.IdExtractor = FirstPathElement
	}
	id = ha.IdExtractor(r.URL.Path)
	// TODO: scan id to ensure it is not malicious
	switch ha.Check(r, id) {
	case AuthDeny:
		// TODO: add WWW-Authenticate header field
		http.Error(w, "401 Unauthorized", http.StatusUnauthorized)
	case AuthNotFound:
		http.Error(w, "404 Not Found", http.StatusNotFound)
	case AuthError:
		http.Error(w, "500 Server Error", http.StatusInternalServerError)
	case AuthAllow:
		if ha.Handler != nil {
			ha.Handler.ServeHTTP(w, r)
		}
	}
}

// FirstPathElement returns the first path component, minus
// any leading or trailing slashes.
func FirstPathElement(s string) string {
	id := strings.TrimPrefix(s, "/")
	// extract up to either the first "/" or the end of the string
	j := strings.Index(id, "/")
	if j != -1 {
		id = id[0:j]
	}
	return id
}

// A RequestUser returns the current user for a request
// It handles verifying cookies and doing any database lookups, if needed.
// It should support concurrent access.
type RequestUser interface {
	User(r *http.Request) User
}

// A User is an identifier and a list of groups which the user belongs to.
// The zero User represents the anonymous user.
type User struct {
	Id     string
	Groups []string
}

type Authorization int

const (
	AuthDeny = iota
	AuthAllow
	AuthNotFound
	AuthError
)

// Check determines whether fedora item id is viewable by the given request.
// Returns true if the item can be viewed; false if the item cannot be viewed.
// The id will be passed to Fedora unaltered, so it should have its prefixes,
// if any, already added. For example,
//	temp:ab12cd34
// instead of
//	ab12cd34
func (ha *HydraAuth) Check(r *http.Request, id string) Authorization {
	log.Printf("Checking rights for %s", id)
	rights := ha.getRights(id)
	if rights == nil {
		log.Printf("Not Found %s", id)
		return AuthNotFound
	}
	var u User // zero is the anon user
	// first try with the anon user to see if item is viewable by the public.
	if rights.canView(u) == AuthAllow {
		log.Printf("Is Public: %s", id)
		return AuthAllow
	}
	// now we need to decode the current user
	if ha.CurrentUser == nil {
		return AuthDeny
	}
	u = ha.CurrentUser.User(r)
	log.Printf("Found user '%s', %#v", u.Id, u.Groups)
	return rights.canView(u)
}

// hydraRights contains the rights associated to a given hydra object.
// It can then be checked against a User
type hydraRights struct {
	readGroups []string
	readPeople []string
	editGroups []string
	editPeople []string
	embargo    time.Time
	version    string
}

// Does this hydraRights allow public viewing?
// Duplicates some of the canView logic to try to prevent decoding the user
// when the decoding isn't needed.
func (hr *hydraRights) isPublic() bool {
	if hr.version != "0.1" {
		return false
	}
	if time.Now().Before(hr.embargo) {
		return false
	}
	if member("public", hr.readGroups) || member("public", hr.editGroups) {
		return true
	}
	return false
}

// Compare an items access rights against a User to see if view access should be
// granted. It will return AuthAllow if the user is allowed to see the item,
// AuthDeny if the user cannot see the item, or one of the other authorization
// codes if there is an error
func (hr *hydraRights) canView(user User) Authorization {
	if hr.version != "0.1" {
		return AuthError
	}
	if time.Now().Before(hr.embargo) {
		// only edit people can view
		if member(user.Id, hr.editPeople) ||
			incommon(user.Groups, hr.editGroups) {
			return AuthAllow
		}
		return AuthDeny
	}

	// public?
	if member("public", hr.readGroups) || member("public", hr.editGroups) {
		return AuthAllow
	}

	// registered?
	if user.Id != "" &&
		(member("registered", hr.readGroups) || member("registered", hr.editGroups)) {
		return AuthAllow
	}
	if incommon(user.Groups, hr.readGroups) || incommon(user.Groups, hr.editGroups) {
		return AuthAllow
	}
	if member(user.Id, hr.readPeople) || member(user.Id, hr.editPeople) {
		return AuthAllow
	}
	return AuthDeny
}

// the []string structures should be replaced by a generic set datatype.
// perhaps a map[string]bool ?

// rightsMetadata is used to decode the hydra rightsMetadata xml data
type rightsMetadata struct {
	Version string           `xml:"version,attr"`
	Access  []accessMetadata `xml:"access"`
	Embargo string           `xml:"embargo>machine>date"`
}

// accessMetadata is used to decode the hydra rightsMetadata xml data
type accessMetadata struct {
	Kind   string   `xml:"type,attr"`
	People []string `xml:"machine>person"`
	Groups []string `xml:"machine>group"`
}

// given an object identifier, get and decode the rights metadata for it
//
// TODO: add a cache with a timed expiry
func (ha *HydraAuth) getRights(id string) *hydraRights {
	r, err := ha.fedora.GetDatastream(id, "rightsMetadata")
	if err != nil {
		log.Println(err)
		return nil
	}
	defer r.Close()

	var rights rightsMetadata
	d := xml.NewDecoder(r)
	err = d.Decode(&rights)
	if err != nil {
		log.Printf("Decode error %s", err.Error())
		return nil
	}

	log.Printf("Decode %s rights: %v", id, rights)

	result := &hydraRights{version: rights.Version}
	for i := range rights.Access {
		switch rights.Access[i].Kind {
		case "read":
			result.readGroups = append(result.readGroups, rights.Access[i].Groups...)
			result.readPeople = append(result.readPeople, rights.Access[i].People...)
		case "edit":
			result.editGroups = append(result.editGroups, rights.Access[i].Groups...)
			result.editPeople = append(result.editPeople, rights.Access[i].People...)
		}
	}

	if rights.Embargo != "" {
		t, err := time.Parse("2006-01-02", rights.Embargo)
		if err != nil {
			log.Printf("Error decoding %s: %v", rights.Embargo, err)
		}
		result.embargo = t
	}

	return result
}
