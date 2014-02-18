package disseminator

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"net/url"

	// I know. Two different libraries just to decode a rails cookie.
	// One is to verify and decrypt the cookie, the other is to unmarshal
	// the data. This unmarshaling library is fragile, but it works well
	// enough until we serialize the session to JSON.
	"github.com/adeven/gorails/marshal"
	"github.com/mattetti/goRailsYourself/crypto"
)

// DeviseAuth will only pass on requests which have a valid RoR devise cookie.
// At the moment it only handles Rails 3 signed cookies.
// Next would be to authenticate Rails 4 encrypted cookies.
// It expects the user id number to be in the session, and that the session does
// not have a timeout.
// The user id is passed to the LookupUser interface to do any database or LDAP
// lookups necessary to turn the id number into a User struct.
type DeviseAuth struct {
	SecretBase []byte     // base secret for verifying the cookie
	CookieName string     // the name of the auth cookie
	Lookup     LookupUser // procedure to turn user id to a User
	verifier   *crypto.MessageVerifier
}

// Given a user id number, look up the user in the database.
// A user object is returned. If the user doesn't exist or there is some other
// error, then the zero user is returned. If there is some kind of error except
// for the user not existing, return an error message in addition to returning
// the zero user.
type LookupUser interface {
	Lookup(uid int) (User, error)
}

func (d *DeviseAuth) User(r *http.Request) User {
	if d.Lookup == nil {
		log.Printf("ERROR lookup is nil")
		return User{}
	}
	cookie, err := r.Cookie(d.CookieName)
	if err != nil {
		return User{}
	}
	session, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return User{}
	}
	uid := d.decodeUserId(session)
	if uid < 0 {
		return User{}
	}
	// lookup user in database
	u, err := d.Lookup.Lookup(uid)
	if err != nil {
		log.Printf("ERROR looking up user %d: %s", uid, err)
	}
	return u
}

// rails 4 cookies are not the easiest thing to work with.
// The following sites were very helpful:
//
// http://matt.aimonetti.net/posts/2013/11/30/sharing-rails-sessions-with-non-ruby-apps/
// http://big-elephants.com/2014-01/handling-rails-4-sessions-with-go/

// decodeUserId takes the session cookie and verifies its signature.
// It then decodes the session data and returns the user's id number.
// In case of error, or the cookie is not authenticated an integer < 0 is returned.
func (d *DeviseAuth) decodeUserId(session string) int {
	if d.verifier == nil {
		d.verifier = &crypto.MessageVerifier{
			Secret:     d.SecretBase,
			Serializer: crypto.NullMsgSerializer{},
		}
		// FIXME: Probably don't need this
		if ok, err := d.verifier.IsValid(); !ok {
			log.Println(err)
			return -1
		}
	}
	// verify cookie
	var decoded string
	err := d.verifier.Verify(session, &decoded)
	if err != nil {
		log.Println(err)
		return -1
	}
	uid, err := unmarshalDevise([]byte(decoded))
	log.Printf("Found user %d", uid)
	return int(uid)
}

// unmarshalDevise returns the user id number for the given serialized data.
// returns a number < 0 in case of error, along with the error
func unmarshalDevise(decryptedSessionData []byte) (int, error) {
	sessionData, err := marshal.CreateMarshalledObject(decryptedSessionData).GetAsMap()
	if err != nil {
		return -1, err
	}
	wardenData, ok := sessionData["warden.user.user.key"]
	if !ok {
		return -2, errors.New("No warden.user.user.key")
	}
	wardenUserKey, err := wardenData.GetAsArray()
	if err != nil || len(wardenUserKey) < 1 {
		if err == nil {
			err = errors.New("warden.user.user.key wrong size")
		}
		return -3, err
	}
	userData, err := wardenUserKey[0].GetAsArray()
	if err != nil || len(userData) < 1 {
		if err == nil {
			err = errors.New("warden.user.user.key first element wrong size")
		}
		return -4, err
	}
	uid, err := userData[0].GetAsInteger()
	if err != nil {
		return -5, err
	}
	return int(uid), nil
}

type DatabaseUser struct {
	Db *sql.DB
}

// Given a user id number, look up the user in the database.
// A user object is returned.
// If the user doesn't exist or there is some other error, then
// the zero user is returned.
func (d *DatabaseUser) Lookup(uid int) (User, error) {
	var username, groups sql.NullString
	row := d.Db.QueryRow("SELECT username, group_list FROM users WHERE id=?", uid)
	err := row.Scan(&username, &groups)
	switch {
	case err == sql.ErrNoRows:
		// no such user
		log.Printf("no user %d", uid)
		return User{}, nil
	case err != nil:
		log.Printf("Database Error: %s", err)
		return User{}, err
	}
	if username.Valid {
		var groupList []string
		if groups.Valid {
			// split out groups
			groupList = []string{groups.String}
		}
		return User{
			Id:     username.String,
			Groups: groupList,
		}, nil
	}
	log.Printf("User %d has no username", uid)
	return User{}, nil
}
