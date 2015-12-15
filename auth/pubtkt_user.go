package auth

import (
	"crypto"
	"crypto/dsa"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ndlib/disadis/timecache"
)

// TODO: add better logging to help track down ticket errors?

// NewPubtktAuth creates a pubtkt authorization from an arbitrary data buffer.
// It is suggested to use NewPubtktAuthFromKeyFile.
func NewPubtktAuth(publicKey interface{}) *PubtktAuth {
	return &PubtktAuth{
		publicKey: publicKey,
		cache:     timecache.New(100, 24*time.Hour),
	}
}

// NewPubtktAuthFromKeyFile takes the name of a PEM public key file
func NewPubtktAuthFromKeyFile(filename string) *PubtktAuth {
	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	buf, err := ioutil.ReadAll(f)
	f.Close()
	if err != nil {
		panic(err)
	}
	return NewPubtktAuthFromPEM(buf)
}

// NewPubtktAuthFromPEM takes a PEM encoded block giving the public key to use
// for decoding.
func NewPubtktAuthFromPEM(pemtext []byte) *PubtktAuth {
	p, _ := pem.Decode(pemtext)
	if p == nil {
		panic("no pem block found")
	}
	key, err := x509.ParsePKIXPublicKey(p.Bytes)
	if err != nil {
		panic(err)
	}
	return NewPubtktAuth(key)
}

// PubtktAuth implements the RequestUser interface.
// Use NewPubtktAuthFromPEM or NewPubtktAuthFromKeyFile to create instances
// of this type
type PubtktAuth struct {
	publicKey interface{}
	cache     timecache.Cache
}

// User returns the user associated with the current request, using pubtkt authentication
func (pa *PubtktAuth) User(r *http.Request) User {
	cookie, err := r.Cookie("auth_pubtkt")
	if err != nil {
		return User{}
	}
	pubtkt, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		log.Printf("Error unescaping cookie %s", err)
	}

	log.Printf("Found pubtkt %s", pubtkt)

	// verify the ticket
	// only valid tickets are put in the cache
	var t *Pubtkt
	v, err := pa.cache.Get(pubtkt)
	if err == nil {
		var ok bool
		t, ok = v.(*Pubtkt)
		if !ok {
			log.Printf("Error casting Pubtkt from cache")
			return User{}
		}
	} else {
		i := strings.LastIndex(pubtkt, ";sig=")
		if i == -1 || !pa.verifySig(pubtkt[:i], pubtkt[i+5:]) {
			log.Printf("ticket sig failed")
			return User{}
		}

		t = parseTicket(pubtkt[:i])
		pa.cache.Add(pubtkt, t)
	}

	if !verifyTicket(r, t) {
		return User{}
	}

	return User{ID: t.UID, Groups: t.Tokens}
}

// verify the message text against signature using the public key
// in PubtktAuth. Expects the signature to be base64 encoded.
// Returns true if the signature is valid, false otherwise
func (pa *PubtktAuth) verifySig(text, signature string) bool {
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		log.Println("problem decoding sig", err)
		return false
	}
	h := sha1.New()
	h.Write([]byte(text))
	digest := h.Sum(nil)

	// This is inspired by the crypto/x509 standard library
	switch pub := pa.publicKey.(type) {
	case *rsa.PublicKey:
		return nil == rsa.VerifyPKCS1v15(pub, crypto.SHA1, digest, sig)
	case *dsa.PublicKey:
		dsaSig := new(dsaSignature)
		if _, err := asn1.Unmarshal(sig, dsaSig); err != nil {
			// log.Println("problem decoding dsa", err)
			return false
		}
		if dsaSig.R.Sign() <= 0 || dsaSig.S.Sign() <= 0 {
			// log.Println("509: DSA signature contained zero or negative values")
			return false
		}
		return dsa.Verify(pub, digest, dsaSig.R, dsaSig.S)
	}

	return false
}

type dsaSignature struct {
	R, S *big.Int
}

// Pubtkt holds the decoded contents of a pubtkt
type Pubtkt struct {
	UID         string
	ClientIP    string
	ValidUntil  time.Time
	GracePeriod time.Time
	Bauth       string
	Tokens      []string
	UData       string
}

// Verify the pairing of the request and the ticket.
// returns true if the pair are valid, false otherwise
func verifyTicket(r *http.Request, t *Pubtkt) bool {
	if t.ClientIP != "" {
		var ip = r.RemoteAddr
		i := strings.Index(ip, ":")
		if i != -1 {
			ip = ip[:i]
		}
		if t.ClientIP != ip {
			log.Println("client ip does not match ticket ip")
			return false
		}
	}
	if time.Now().After(t.ValidUntil) {
		log.Println("ticket has expired")
		return false
	}
	return true
}

// parses the text after the ';sig=...' has been removed from the end
// Returns a Pubtkt structure, or nil if there was a parse error
//
// a pubtkt consists of a sequence of fields in the form "key=value"
// separated by semicolons.
func parseTicket(text string) *Pubtkt {
	var result = new(Pubtkt)
	fields := strings.Split(text, ";")
	for i := range fields {
		kv := strings.SplitN(fields[i], "=", 2)
		if len(kv) != 2 {
			// malformed key-value pair, skip
			continue
		}
		if kv[1] == "" {
			continue
		}
		switch kv[0] {
		case "uid":
			result.UID = kv[1]
		case "cip":
			result.ClientIP = kv[1]
		case "validuntil":
			z, err := strconv.Atoi(kv[1])
			if err == nil {
				result.ValidUntil = time.Unix(int64(z), 0)
			}
		case "graceperiod":
			z, err := strconv.Atoi(kv[1])
			if err == nil {
				result.GracePeriod = time.Unix(int64(z), 0)
			}
		case "bauth":
			result.Bauth = kv[1]
		case "tokens":
			result.Tokens = strings.Split(kv[1], ",")
		case "udata":
			result.UData = kv[1]
		}
	}
	//log.Printf("Decoded ticket %v", result)
	return result
}
