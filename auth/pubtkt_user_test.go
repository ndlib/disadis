package auth

import (
	"fmt"
	"testing"
	"time"
)

func TestParseTicket(t *testing.T) {
	var table = []struct {
		s string
		u Pubtkt
	}{
		{s: "uid=yyy;validuntil=123456789;tokens=;udata=;sig=xxx",
			u: Pubtkt{UID: "yyy", ValidUntil: time.Unix(123456789, 0)}},
		{s: "uid=yyy;validuntil=123456789;sig=xxx",
			u: Pubtkt{UID: "yyy", ValidUntil: time.Unix(123456789, 0)}},
		{s: "uid=yyy;validuntil=123456789;tokens=a,b,c;sig=xxx",
			u: Pubtkt{UID: "yyy",
				ValidUntil: time.Unix(123456789, 0),
				Tokens:     []string{"a", "b", "c"}}},
	}

	for i := range table {
		var (
			u   = parseTicket(table[i].s)
			v   = table[i].u
			msg = ""
		)
		switch {
		case u.UID != v.UID:
			msg = "UID"
		case u.ClientIP != v.ClientIP:
			msg = "ClientIP"
		case u.ValidUntil != v.ValidUntil:
			msg = "ValidUntil"
		case u.GracePeriod != v.GracePeriod:
			msg = "GracePeriod"
		case u.Bauth != v.Bauth:
			msg = "Bauth"
		case u.UData != v.UData:
			msg = "UData"
		case len(u.Tokens) != len(v.Tokens):
			msg = fmt.Sprintf("Token length %d != %d", len(u.Tokens), len(v.Tokens))
		default:
			for j := range u.Tokens {
				if u.Tokens[j] != v.Tokens[j] {
					msg = "Tokens"
					break
				}
			}
		}
		if msg != "" {
			t.Errorf("%s decode error %s: %v != %v", table[i].s, msg, u, v)
		}
	}

}

/*
  dsa_private_key = `-----BEGIN DSA PRIVATE KEY-----
MIIBuwIBAAKBgQD8SBex5jpx45TqNwuAklFjgNUxa60fGsjYWIE/eTnLDF5WmkBx
el4Iz0xaEI6EOBplHEPagbpqvvFnFcnUK3+4t+jq57D/4NVwXFJHU++qgQMu7K84
N0m9HNiRhM3XEr1v5RlfmUCUM6vDROfhEWWwgxfKDrSQi//yZ27PGkbVMwIVAMx5
+7JPX5tzhoXXNC4IjKTbnL1rAoGAGJQSCtMFKFi90k2FAOg7vhp7Fwu0IwmnXRIM
t+jcNTncxkUtklhWUR20ZJ6t1hq63UPquwZ4XYD45MHNoXyXR0lVEZe74lK9Xzmm
OImBuYJdBiN75RbjoWjyy2KVmklCylDGvkXsF/o5LDi2aMxtcOYkf41JZDFVG/n+
xL5Z6ScCgYEA9T1el54DlVN7F4OBGwIxGwnpsjt5mcgFeFbYM53tjTkgljkrLWmm
WwTt9A650taMweCOp+T/L2c6gnZa7abKCWZjfBBNwoK/v4IKMwDvmRcw275lvWTl
FL3HVK9QVHv8fYXg1oQ/05DI2aDuCmDUp5Jk6ePl7B5glZiSoJZYUkMCFGDlMEur
j7ndb1DQy5mHeqWskXwL
-----END DSA PRIVATE KEY-----
`
*/

var (
	dsaPublicKey = `-----BEGIN PUBLIC KEY-----
MIIBtzCCASsGByqGSM44BAEwggEeAoGBAPxIF7HmOnHjlOo3C4CSUWOA1TFrrR8a
yNhYgT95OcsMXlaaQHF6XgjPTFoQjoQ4GmUcQ9qBumq+8WcVydQrf7i36OrnsP/g
1XBcUkdT76qBAy7srzg3Sb0c2JGEzdcSvW/lGV+ZQJQzq8NE5+ERZbCDF8oOtJCL
//Jnbs8aRtUzAhUAzHn7sk9fm3OGhdc0LgiMpNucvWsCgYAYlBIK0wUoWL3STYUA
6Du+GnsXC7QjCaddEgy36Nw1OdzGRS2SWFZRHbRknq3WGrrdQ+q7BnhdgPjkwc2h
fJdHSVURl7viUr1fOaY4iYG5gl0GI3vlFuOhaPLLYpWaSULKUMa+RewX+jksOLZo
zG1w5iR/jUlkMVUb+f7EvlnpJwOBhQACgYEA9T1el54DlVN7F4OBGwIxGwnpsjt5
mcgFeFbYM53tjTkgljkrLWmmWwTt9A650taMweCOp+T/L2c6gnZa7abKCWZjfBBN
woK/v4IKMwDvmRcw275lvWTlFL3HVK9QVHv8fYXg1oQ/05DI2aDuCmDUp5Jk6ePl
7B5glZiSoJZYUkM=
-----END PUBLIC KEY-----
`
	textDSA = `uid=foobar;validuntil=123456789;tokens=;udata=`
	sigDSA  = `MCwCFEmMvKWbbIjTCJMbgz1P4N+TWOodAhRRTr9odvBjKtCKaE1B6ysW548oqw==`
	sigBad  = `01234567890123456789012345678901234567890123456789012345678912==`
)

func TestVerify(t *testing.T) {
	pa := NewPubtktAuthFromPEM([]byte(dsaPublicKey))
	if !pa.verifySig(textDSA, sigDSA) {
		t.Errorf("ticket failed verification")
	}
	if pa.verifySig(textDSA+"1", sigDSA) {
		t.Errorf("changed ticket passed verification")
	}
	if pa.verifySig(textDSA, sigBad) {
		t.Errorf("ticket passed with bad signature")
	}
}
