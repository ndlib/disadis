package disseminator

import (
	"encoding/xml"
	"log"
	"net/http"
	"time"
)

func NewPermitHydra(fedoraPath, namespace string) *hydraAuth {
	return &hydraAuth{
		fedoraPrefix: fedoraPath + "objects/" + namespace,
	}
}

type hydraAuth struct {
	fedoraPrefix string
}

func (ha *hydraAuth) Check(r *http.Request, id string, isThumb bool) bool {
	if isThumb {
		return true
	}

	rights := ha.getRights(id)
	if rights == nil {
		return false
	}
	return rights.canView("", nil)
}

type hydraRights struct {
	readGroups []string
	readPeople []string
	editGroups []string
	editPeople []string
	embargo    time.Time
	version    string
}

func (hr *hydraRights) canView(user string, groups []string) bool {
	if hr.version != "0.1" {
		return false
	}
	if time.Now().Before(hr.embargo) {
		// only edit people can view
		if member(user, hr.editPeople) ||
			incommon(groups, hr.editGroups) {
			return true
		}
		return false
	}

	// public?
	if member("public", hr.readGroups) || member("public", hr.editGroups) {
		return true
	}

	// registered?
	if user != "" && (member("registered", hr.readGroups) || member("registered", hr.editGroups)) {
		return true
	}

	if incommon(groups, hr.readGroups) || incommon(groups, hr.editGroups) {
		return true
	}
	if member(user, hr.readPeople) || member(user, hr.editPeople) {
		return true
	}

	return false
}

func member(a string, list []string) bool {
	for i := range list {
		if a == list[i] {
			return true
		}
	}
	return false
}

func incommon(a, b []string) bool {
	for i := range a {
		if member(a[i], b) {
			return true
		}
	}
	return false
}

type rightsMetadata struct {
	Version string           `xml:"version,attr"`
	Access  []accessMetadata `xml:"access"`
	Embargo string           `xml:"embargo>machine>date"`
}

type accessMetadata struct {
	Kind   string   `xml:"type,attr"`
	People []string `xml:"machine>person"`
	Groups []string `xml:"machine>group"`
}

func (ha *hydraAuth) getRights(id string) *hydraRights {
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
