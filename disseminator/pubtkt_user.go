package disseminator

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
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func NewPubtktAuth(publicKey interface{}) *PubtktAuth {
	return &PubtktAuth{publicKey: publicKey}
}

func NewPubtktAuthFromKeyFile(filename string) *PubtktAuth {
	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	buf, err := ioutil.ReadAll(f)
	if err != nil {
		panic(err)
	}
	return NewPubtktAuthFromPEM(buf)
}

func NewPubtktAuthFromPEM(pemtext []byte) *PubtktAuth {
	p, _ := pem.Decode(pemtext)
	if p == nil {
		panic("no pem block found")
	}
	key, err := x509.ParsePKIXPublicKey(p.Bytes)
	if err != nil {
		panic(err)
	}
	return &PubtktAuth{publicKey: key}
}

type PubtktAuth struct {
	publicKey interface{}
}

func (pa *PubtktAuth) User(r *http.Request) User {
	cookie, err := r.Cookie("auth_pubtkt")
	if err != nil {
		return User{}
	}
	pubtkt := cookie.Value

	// verify the ticket
	i := strings.LastIndex(pubtkt, ";sig=")
	if i == -1 || !pa.verifySig(pubtkt[:i], pubtkt[i+5:]) {
		return User{}
	}

	t := parseTicket(pubtkt[:i])

	if !verifyTicket(r, t) {
		return User{}
	}

	return User{Id: t.Uid, Groups: t.Tokens}
}

// verify the message text against signature using the public key
// in PubtktAuth. The signature is base64 encoded.
// Returns true if the signature is valid, false otherwise
func (pa *PubtktAuth) verifySig(text, signature string) bool {
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
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
			return false
		}
		if dsaSig.R.Sign() <= 0 || dsaSig.S.Sign() <= 0 {
			// "x509: DSA signature contained zero or negative values")
			return false
		}
		return dsa.Verify(pub, digest, dsaSig.R, dsaSig.S)
	}

	return false
}

type dsaSignature struct {
	R, S *big.Int
}

type Pubtkt struct {
	Uid         string
	ClientIP    string
	ValidUntil  time.Time
	GracePeriod time.Time
	Bauth       string
	Tokens      []string
	UData       string
}

// verifies the pairing of the request and the ticket
// returns true if the pair are valid, false otherwise
func verifyTicket(r *http.Request, t *Pubtkt) bool {
	if t.ClientIP != "" {
		var ip = r.RemoteAddr
		i := strings.Index(ip, ":")
		if i != -1 {
			ip = ip[:i]
		}
		if t.ClientIP != ip {
			return false
		}
	}
	if time.Now().After(t.ValidUntil) {
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
			result.Uid = kv[1]
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
	return result
}
