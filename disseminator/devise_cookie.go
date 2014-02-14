package disseminator

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"net/url"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"

	// I know. Two different libraries just to decode a rails cookie.
	// One is to verify and decrypt the cookie, the other is to unmarshal
	// the data. This unmarshaling library is fragile, but it works well
	// enough until we serialize the session to JSON.
	"github.com/adeven/gorails/marshal"
	"github.com/mattetti/goRailsYourself/crypto"
)

var (
	unauthorizedUser = errors.New("Unauthorized user")
	invalidAuthData  = errors.New("Invalid auth data")
)

type DeviseAuth struct {
	Db         *sql.DB
	SecretBase []byte
	CookieName string
	verifier   *crypto.MessageVerifier
}

func (d *DeviseAuth) User(r *http.Request) User {
	cookie, err := r.Cookie(d.CookieName)
	if err != nil {
		return User{Id: err.Error()}
	}
	session, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return User{Id: err.Error()}
	}
	uid := d.decodeUserId(session)
	if uid < 0 {
		return User{}
	}
	if uid == 0 {
		// BIG Problem. The rest of the code takes user number 0
		// to be the anonymous user.
		log.Printf("Found user number 0")
	}

	// lookup user in database
	return d.lookupUser(uid)

}

// Given a user id number, look up the user in the database.
// A user object is returned.
// If the user doesn't exist or there is some other error, then
// the zero user is returned.
func (d *DeviseAuth) lookupUser(uid int) User {
	var username, groups string
	row := d.Db.QueryRow("SELECT username, groups FROM User WHERE uid=?", uid)
	err := row.Scan(&username, &groups)
	switch {
	case err == sql.ErrNoRows:
		// no such user
		return User{}
	case err != nil:
		log.Printf("Database Error: %s", err)
		return User{}
	default:
		return User{
			Id:     username,
			Groups: []string{groups},
		}
	}
}

// rails 4 cookies are not the easiest thing to work with.
// The following sites were very helpful:
//
// http://matt.aimonetti.net/posts/2013/11/30/sharing-rails-sessions-with-non-ruby-apps/
// http://big-elephants.com/2014-01/handling-rails-4-sessions-with-go/

// decodeUserId takes the session cookie and verifies its signature.
// It then decodes the session data and returns the user's id number.
// In case of error, or the cookie not authentication -1 is returned.
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
	if err != nil {
		return -1
	}
	log.Printf("Found user %d", uid)
	return int(uid)
}

// unmarshalDevise returns the user id number for the given serialized data.
func unmarshalDevise(decryptedSessionData []byte) (int64, err) {
	sessionData, err := marshal.CreateMarshalledObject(decryptedSessionData).GetAsMap()
	if err != nil {
		return 0, err
	}
	wardenData, ok := sessionData["warden.user.user.key"]
	if !ok {
		return -1
	}
	wardenUserKey, err := wardenData.GetAsArray()
	if err != nil || len(wardenUserKey) < 1 {
		return -1
	}
	userData, err := wardenUserKey[0].GetAsArray()
	if err != nil || len(userData) < 1 {
		return -1
	}
	return userData[0].GetAsInteger()
}
