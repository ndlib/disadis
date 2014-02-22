package disseminator

import (
	"database/sql"
	"fmt"
	"net/http"
	"testing"

	// using this library makes the tests fail. Is there something with QueryRow?
	//_ "github.com/mattn/go-sqlite3"

	// yet this sqlite library has the tests passing
	_ "code.google.com/p/go-sqlite/go1/sqlite3"
)

func TestDevise(t *testing.T) {
	var d = &DeviseAuth{
		SecretBase: []byte("0123456789abcdefghijklmnopqrstuvwxyz"),
		CookieName: "_test_session",
		Lookup:     &NoLookup{},
	}
	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Errorf("%s", err)
		return
	}
	cookie := http.Cookie{
		Name:  "_test_session",
		Value: `BAh7DEkiD3Nlc3Npb25faWQGOgZFVEkiJTU1YWE3OWU1MTE2ZTkyMzM4ZWMxYmJlNzNkMjlkM2Q5BjsAVEkiC3NlYXJjaAY7AEZ7AEkiDGhpc3RvcnkGOwBGWwBJIhpjYXNfbGFzdF92YWxpZF90aWNrZXQGOwBUSSIiU1QtMTg4MTgtbFNQckZ0ZFQ5WW1GUVA2aWM1dE4GOwBUSSIgY2FzX2xhc3RfdmFsaWRfdGlja2V0X3N0b3JlBjsAVEZJIhl3YXJkZW4udXNlci51c2VyLmtleQY7AFRbB1sGaQYwSSIQX2NzcmZfdG9rZW4GOwBGSSIxR3RWZ2hocG5pZjhqWUVPVDAveTlXTzZqUS8vZmJBK2pjREtjT2tWUTlDZz0GOwBG--2dd592ed4b4e7384c2febd20b0c4a3ff3629231c`,
	}
	req.AddCookie(&cookie)
	u := d.User(req)
	if u.Id != "User-1" {
		t.Errorf("Got user %s", u.Id)
	}
}

type NoLookup struct{}

func (n *NoLookup) Lookup(uid int) (User, error) {
	return User{Id: fmt.Sprintf("User-%d", uid)}, nil
}

const dbSchema = `CREATE TABLE users (
id INTEGER,
username STRING,
group_list STRING
);
INSERT INTO users VALUES (0, "king", null);
INSERT INTO users VALUES (1, "queen", null);
INSERT INTO users VALUES (3, null, null);
`

func TestDatabaseLookup(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Skip(err)
		return
	}
	defer db.Close()

	_, err = db.Exec(dbSchema)
	if err != nil {
		t.Log(err)
	}
	d := &DatabaseUser{Db: db}

	var username sql.NullString
	row := db.QueryRow("SELECT username FROM users WHERE uid=?", 0)
	err = row.Scan(&username)
	if err != nil {
		t.Logf("err = %s", err)
	} else {
		t.Logf("username = %#v", username)
	}

	table := []struct {
		uid int
		u   User
	}{
		{0, User{"king", nil}},
		{1, User{"queen", nil}},
		{1000, User{"", nil}},
	}

	for i, item := range table {
		u, _ := d.Lookup(item.uid)
		if u.Id != item.u.Id {
			t.Errorf("User id %d returned %#v", item.uid, u)
		}
		if len(u.Groups) != len(item.u.Groups) {
			t.Errorf("Wrong group lengths (index %d)", i)
		}
	}
}

func TestUnmarshal(t *testing.T) {
	table := []struct {
		v string
		n int
	}{
		// These are the raw marshal values. They are not base64 encoded.
		{v: "\x04\b{\bI\"\x0Fsession_id\x06:\x06ETI\"%491b3f98671c8c5cd6409c4371092efe\x06;\x00TI\"\x19warden.user.user.key\x06;\x00T[\a[\x06i\x060I\"\x10_csrf_token\x06;\x00FI\"\bZZZ\x06;\x00T",
			n: 1},
		{v: "\x04\b{\bI\"\x0Fsession_id\x06:\x06ETI\"%491b3f98671c8c5cd6409c4371092efe\x06;\x00TI\"\x19warden.user.user.key\x06;\x00T[\a[\x06i\x02\xDB\x030I\"\x10_csrf_token\x06;\x00FI\"\bZZZ\x06;\x00T",
			n: 987},
	}

	for i, item := range table {
		t.Logf("v = %s", item.v)
		n, err := unmarshalDevise([]byte(item.v))
		if n != item.n {
			t.Errorf("Ticket %d returned wrong user id (%d instead of %d)", i, n, item.n)
			t.Error(err)
		}
	}
}
