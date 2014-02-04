package disseminator

import (
	"encoding/xml"
	"log"
	"net/http"
	"time"
)

func NewHydraAuth(fedoraPath, namespace string) *HydraAuth {
	return &HydraAuth{
		fedoraPrefix: fedoraPath + "objects/" + namespace,
	}
}

type HydraAuth struct {
	CurrentUser RequestUser	// determines the current user
	fedoraPrefix string	// location of fedora along with username and password
}


// userer returns the current user for a request
// You probably want to use the RequestUser interface
type userer interface {
	User() User
}

// A Userer returns the current user for a request
// It handles verifying cookies and doing any database lookups, if needed
type RequestUser interface {
	User(r *http.Request) User
}


// A User is an identifier and a list of groups which the user belongs to.
// The zero User represents an anonymous user.
type User struct {
	Id string
	Groups []string
}

// See if the item `id` is viewable by the current user in the request.
// Returns true if the item can be viewed; false if the item cannot be viewed.
//
// The isThumb flag seems like a hack. Is there a better way? maybe have
// Check reparse the request path?
func (ha *HydraAuth) Check(r *http.Request, id string, isThumb bool) bool {
	if isThumb {
		return true
	}

	rights := ha.getRights(id)
	if rights == nil {
		return false
	}
	var lu = lazyUser{f: ha.CurrentUser, r: r}
	return rights.canView(&lu)
}

type lazyUser struct {
	f CurrentUser
	r *http.Request
}

func (lu *lazyUser) User() User {
	if lu.f == nil {
		// if a UserFinder was never supplied, return the anon user
		return User{}
	}
	return lu.f.User(lu.r)
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

// Compare an items access rights against a User to see if view access should be
// granted. It will return either `true' if the user is allowed to see the item
// or 'false' if the user cannot see the item
//
// A lazyUser is used instead of a User since canView() will only do a user lookup
// if the item is not viewable by the public. A side-effect of this is that we
// do not track who is downloading public content.
func (hr *hydraRights) canView(u userer) bool {
	if hr.version != "0.1" {
		return false
	}
	if time.Now().Before(hr.embargo) {
		user := u.User()

		// only edit people can view
		if member(user.Id, hr.editPeople) ||
			incommon(user.Groups, hr.editGroups) {
			return true
		}
		return false
	}

	// public?
	if member("public", hr.readGroups) || member("public", hr.editGroups) {
		return true
	}

	user := u.User()

	// registered?
	if user.Id != "" && (member("registered", hr.readGroups) || member("registered", hr.editGroups)) {
		return true
	}

	if incommon(user.Groups, hr.readGroups) || incommon(user.Groups, hr.editGroups) {
		return true
	}
	if member(user.Id, hr.readPeople) || member(user.Id, hr.editPeople) {
		return true
	}

	return false
}

// the []string structures should be replaced by a generic set datatype.
// perhaps a map[string]bool ?

// is string 'a' a member of string list 'list'?
func member(a string, list []string) bool {
	for i := range list {
		if a == list[i] {
			return true
		}
	}
	return false
}

// do lists 'a' and 'b' contain a member in common?
func incommon(a, b []string) bool {
	for i := range a {
		if member(a[i], b) {
			return true
		}
	}
	return false
}

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
	log.Printf("getting rights %s", id)
	r, err := http.Get(ha.fedoraPrefix + id + "/datastreams/rightsMetadata/content")
	if err != nil {
		log.Println(err)
		return nil
	}
	defer r.Body.Close()

	if r.StatusCode != 200 {
		log.Printf("Got status %d from fedora", r.StatusCode)
		return nil
	}

	var rights rightsMetadata
	d := xml.NewDecoder(r.Body)
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
