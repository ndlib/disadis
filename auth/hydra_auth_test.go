package auth

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
		allowed, registered, embargo Authorization
	}{
		{"elephant", nil, AuthAllow, AuthAllow, AuthDeny},                              // read person can read
		{"xerxes", []string{"yak", "carrot"}, AuthAllow, AuthAllow, AuthDeny},          // read group can read
		{"kite", []string{"yak", "water"}, AuthAllow, AuthAllow, AuthAllow},            // edit person can read
		{"xerxes", []string{"yak", "water", "igloo"}, AuthAllow, AuthAllow, AuthAllow}, // edit group can read
		{"xerxes", []string{"kite"}, AuthDeny, AuthAllow, AuthDeny},                    // keep people and groups separate
		{"", nil, AuthDeny, AuthDeny, AuthDeny},                                        // public cannot read yet
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
		if a != AuthAllow {
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
		if a == AuthAllow {
			t.Errorf("got %v with %v\n", a, z)
		}
	}
}
