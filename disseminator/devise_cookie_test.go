package disseminator

import (
	"net/http"
	"testing"
)

func TestDevise(t *testing.T) {

	var d = &DeviseAuth{
		SecretBase: []byte("1597e868fa2dfb2f782fd4477d0e93d4f24c31f2da5d54edac15391eaf60dcd6aeba91856bc12fefc9d9ad329ab023ed1503cf8c51dc88e4ae4040655a92ae8f"),
		CookieName: "_test_session",
	}
	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Errorf("%s", err)
		return
	}
	cookie := http.Cookie{
		Name:  "_test_session",
		Value: `BAh7DEkiD3Nlc3Npb25faWQGOgZFVEkiJWZiN2Q5NGIzZWU2OWVlOWFhNzVlYzU3Y2M1ZTlkZjdlBjsAVEkiC3NlYXJjaAY7AEZ7AEkiDGhpc3RvcnkGOwBGWwBJIhpjYXNfbGFzdF92YWxpZF90aWNrZXQGOwBUSSImU1QtOTQxNTItSnJoYTQxZWNoTzlLNUdqZUcwOXQtY2FzBjsAVEkiIGNhc19sYXN0X3ZhbGlkX3RpY2tldF9zdG9yZQY7AFRGSSIZd2FyZGVuLnVzZXIudXNlci5rZXkGOwBUWwdbBmkKMEkiEF9jc3JmX3Rva2VuBjsARkkiMVY3N1FFTTBKMmlVR3NZQk1yaTkrd1hPeTFrU1l4eTBXMUxRYm9hbjlvdlk9BjsARg==--1326c120c741e0de455aa638e6b8062c0edce25f`,
	}
	req.AddCookie(&cookie)
	u := d.User(req)
	t.Errorf("%s", u.Id)

}
