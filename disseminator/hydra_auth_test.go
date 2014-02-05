package disseminator

import (
	"testing"
	"time"
)

func TestCanView(t *testing.T) {
	var hr = hydraRights{
		version:    "0.1",
		readGroups: []string{"apple", "banana", "carrot"},
		readPeople: []string{"dog", "elephant", "faries"},
		editGroups: []string{"grapes", "hay", "igloo"},
		editPeople: []string{"jerky", "kite", "leek"},
	}
	var table = []struct {
		user                         string
		groups                       []string
		allowed, registered, embargo bool
	}{
		{"elephant", nil, true, true, false},                            // read person can read
		{"xerxes", []string{"yak", "carrot"}, true, true, false},        // read group can read
		{"kite", []string{"yak", "water"}, true, true, true},            // edit person can read
		{"xerxes", []string{"yak", "water", "igloo"}, true, true, true}, // edit group can read
		{"xerxes", []string{"kite"}, false, true, false},                // keep people and groups separate
		{"", nil, false, false, false},                                  // public cannot read yet
	}
	var u User
	for _, z := range table {
		u.Id = z.user
		u.Groups = z.groups
		a := hr.canView(u)
		if a != z.allowed {
			t.Errorf("got %v with %v\n", a, z)
		}
	}

	hr.readGroups = append(hr.readGroups, "registered")
	for _, z := range table {
		u.Id = z.user
		u.Groups = z.groups
		a := hr.canView(u)
		if a != z.registered {
			t.Errorf("got %v with %v\n", a, z)
		}
	}

	hr.readGroups = append(hr.readGroups, "public")
	for _, z := range table {
		u.Id = z.user
		u.Groups = z.groups
		a := hr.canView(u)
		if !a {
			t.Errorf("got %v with %v\n", a, z)
		}
	}

	hr.embargo = time.Now().Add(time.Hour)
	for _, z := range table {
		u.Id = z.user
		u.Groups = z.groups
		a := hr.canView(u)
		if a != z.embargo {
			t.Errorf("got %v with %v\n", a, z)
		}
	}

	hr.version = "0.2"
	for _, z := range table {
		u.Id = z.user
		u.Groups = z.groups
		a := hr.canView(u)
		if a {
			t.Errorf("got %v with %v\n", a, z)
		}
	}
}
