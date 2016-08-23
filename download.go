package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ndlib/disadis/auth"
	"github.com/ndlib/disadis/fedora"
)

// DownloadHandler handles the routes
//
//	GET	/:id
//	HEAD	/:id
//
// And, if Versioned is true, the routes
//
//	GET	/:id/:version
//	HEAD	/:id/:version
//
// The first routes will return current version of the contents of the
// datastream named Ds.
// The second group will either return the current version of the contents of
// Ds, provided the current version is equal to :version. Otherwise,
// a 403 Error is returned.
//
// If Auth is not nil, the object with the given identifier is passed
// to Auth, which may either return an error, a redirect, or nothing.
// If nothing is returned, the contents are passed back.
// The Auth handling is done after the identifier is decoded, but before
// the version check, if any.
//
// The reason the Handler calls Auth directly, instead of presuming
// the auth handler has wrapped this one, is because this handler knows
// how to parse the id out of the url, and it seems easier to just pass
// the id to the auth handler than to have the auth handler do the same
// thing.
//
// A pid namespace prefix can be assigned. It will be prepended to
// any decoded identifiers. Nothing is put between the prefix and the
// id, so include any colons in the prefix. e.g. "vecnet:"
//
// Note that because the identifier is pulled from the URL, identifiers
// containing forward slashes are problematic and are not handled.
// Also, identifiers shorter than 1 or longer than 64 characters are rejected.
// (If this is a problem for you, the limit can be changed).
//
// Example Usage:
//	fedora := "http://fedoraAdmin:fedoraAdmin@localhost:8983/fedora/"
//	dh = NewDownloadHandler(NewRemoteFedora(fedora, ""))
//	dh.Ds = "content"
//	dh.Prefix = "vecnet:"
//	dh.Auth = NewHydraAuth(fedora, "")
//	http.Handle("/d/", http.StripPrefix("/d/", dh))
//	return http.ListenAndServe(":"+port, nil)
type DownloadHandler struct {
	Fedora    fedora.Fedora
	Ds        string
	Versioned bool
	Prefix    string
	Auth      *auth.HydraAuth
}

func notFound(w http.ResponseWriter) {
	http.Error(w, "404 Not Found", http.StatusNotFound)
}

func (dh *DownloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		notFound(w)
		return
	}

	// "" / "id" ( / :version )?
	// :version may contain slashes, if there are more slashes in the url.
	// this way we can verify IIIF requests (which have lots of slashes) easily
	path := strings.TrimPrefix(r.URL.Path, "/")
	path = strings.TrimSuffix(path, "/")
	// will always return a string of length 1 or 2
	components := strings.SplitN(path, "/", 2)

	// will an identifier ever have more than 64 characters?
	if len(components[0]) == 0 || len(components[0]) > 64 {
		notFound(w)
		return
	}

	var (
		pid     = dh.Prefix + components[0] // sanitize pid somehow?
		version = -1                        // -1 == current version
	)
	// auth?
	if dh.Auth != nil {
		switch dh.Auth.Check(r, pid) {
		case auth.AuthDeny:
			// TODO: add WWW-Authenticate header field
			http.Error(w, "401 Unauthorized", http.StatusUnauthorized)
			return
		case auth.AuthNotFound:
			notFound(w)
			return
		case auth.AuthAllow:
			break
		case auth.AuthError:
			fallthrough
		default:
			http.Error(w, "500 Server Error", http.StatusInternalServerError)
			return
		}
	}
	// figure out versions
	if len(components) == 2 && dh.Versioned {
		var err error
		version, err = strconv.Atoi(components[1])
		if err != nil || version < 0 {
			notFound(w)
			return
		}
	}

	// always hit fedora for most recent version
	dsinfo, err := dh.Fedora.GetDatastreamInfo(pid, dh.Ds)
	if err != nil {
		log.Printf("Received Fedora error (%s,%s): %s", pid, dh.Ds, err.Error())
		notFound(w)
		return
	}

	// does the version requested match the current version number?
	if version >= 0 && version != dsinfo.Version() {
		http.Error(w, "403 Forbidden", http.StatusForbidden)
		return
	}

	// return content
	// TODO(dbrower): should we see if the dsinfo.LocationType is "URL"?
	// then we don't need to hit fedora just to get a redirect.
	content, info, err := dh.Fedora.GetDatastream(pid, dh.Ds)
	if err != nil {
		switch err {
		case fedora.ErrNotFound:
			notFound(w)
			return
		default:
			log.Println("Received fedora error:", err)
			http.Error(w, "500 Internal Error", http.StatusInternalServerError)
			return
		}
	}
	defer content.Close()

	// sometimes fedora appends an extra extension. See FCREPO-497 in the
	// fedora commons JIRA. This is why we pull the filename directly from
	// the datastream label.
	w.Header().Set("Content-Disposition", `inline; filename="`+dsinfo.Label+`"`)
	// set content-type from the datastream info instead of the returned header.
	// (since if we redirect to bendo, we get bendo's content-type and bendo has no
	// idea of what it should be)
	w.Header().Set("Content-Type", info.Type)
	w.Header().Set("Content-Type", dsinfo.MIMEType)
	// This is set by ServeContent()
	//w.Header().Set("Content-Length", info.Length)
	w.Header().Set("Content-Transfer-Encoding", "binary")
	w.Header().Set("Cache-Control", "private")
	w.Header().Set("ETag", `"`+dsinfo.VersionID+`"`)

	// use ServeContent and the StreamSeeker to handle range requests.
	// when/if fedora ever supports range requests, this should be changed to
	// pass the range through
	n, _ := strconv.ParseInt(info.Length, 10, 64)
	http.ServeContent(w, r, dsinfo.Label, time.Time{}, NewStreamSeeker(content, n))
	return
}
