package main

import (
	"fmt"
	"io"
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
	Fedora                fedora.Fedora   // connection to fedora
	Ds                    string          // the datastream to proxy
	Versioned             bool            // True if we support versioned paths
	Prefix                string          // the PID prefix to use, needs colon
	Auth                  *auth.HydraAuth // kept for vecnet
	BendoToken            string          // optional, used for 'E' and 'R' datastreams
	Aws_access_key        string          // AWS KEY for S3 bucket access
	Aws_secret_access_key string          //SECRET for AWS S3 key
	Aws_region            string          // AWS region
	Aws_s3_bucket_subdir  string          // format s3:/bucketname/bucket_subdir
}

func (dh *DownloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
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
		http.NotFound(w, r)
		return
	}

	pid := dh.Prefix + components[0] // sanitize pid somehow?

	// auth?
	if dh.Auth != nil {
		switch dh.Auth.Check(r, pid) {
		case auth.AuthDeny:
			// TODO: add WWW-Authenticate header field
			http.Error(w, "401 Unauthorized", http.StatusUnauthorized)
			return
		case auth.AuthNotFound:
			http.NotFound(w, r)
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

	// always hit fedora for most recent info
	// Should this lookup be cached?
	dsinfo, err := dh.Fedora.GetDatastreamInfo(pid, dh.Ds)
	if err != nil {
		log.Printf("Received Fedora error (%s,%s): %s", pid, dh.Ds, err.Error())
		http.NotFound(w, r)
		return
	}

	// Figure out versions, if the feature is enabled.
	// We only allow the download of the most recent version.
	// If a version was not passed in the URL (i.e. len(components) == 1)
	// then we take that to mean the most recent version and skip the check.
	if len(components) == 2 && dh.Versioned {
		version, err := strconv.Atoi(components[1])
		if err != nil || version < 0 {
			http.NotFound(w, r)
			return
		}
		if version != dsinfo.Version() {
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
	}

	// short circuit the e-tag check before trying to get content from the source
	// This is simplistic to handle the common case early.
	if haveEtag := r.Header.Get("If-None-Match"); haveEtag != "" {
		etag := `"` + dsinfo.VersionID + `"`
		if haveEtag == etag {
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	// return content
	var content io.ReadCloser
	var info fedora.ContentInfo
	if dh.BendoToken != "" && dsinfo.LocationType == "URL" {
		// this datastream is stored outside of fedora
		// Get the content directly. This way we can supply the auth headers
		// directly to the content supplier.
		content, info, err = getBendoContent(dsinfo.Location, dh.BendoToken)
	} else {
		// get the content from fedora
		content, info, err = dh.Fedora.GetDatastream(pid, dh.Ds)
	}
	if err != nil {
		switch err {
		case fedora.ErrNotFound:
			http.NotFound(w, r)
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
	w.Header().Set("Content-Type", dsinfo.MIMEType)
	// This is set by ServeContent()
	//w.Header().Set("Content-Length", info.Length)
	w.Header().Set("Content-Transfer-Encoding", "binary")
	w.Header().Set("Cache-Control", "private")
	w.Header().Set("ETag", `"`+dsinfo.VersionID+`"`)

	// Use the size returned from the content request in case we redirected
	n, _ := strconv.ParseInt(info.Length, 10, 64)
	if n <= 0 {
		if r.Method == "HEAD" {
			return
		}
		// We have no idea of the content length...
		// so we don't support range requests
		_, err = io.Copy(w, content)
		if err != nil {
			log.Println(err)
		}
		return
	}

	// use ServeContent and the StreamSeeker to handle range requests.
	// when/if fedora ever supports range requests, this should be changed to
	// pass the range through
	http.ServeContent(w, r, dsinfo.Label, time.Time{}, NewStreamSeeker(content, n))
}

// returns the contents of the given URL
// The returned stream needs to be closed when finished.
func getBendoContent(url, token string) (io.ReadCloser, fedora.ContentInfo, error) {
	var info fedora.ContentInfo
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, info, err
	}
	req.Header.Add("X-Api-Key", token)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, info, err
	}
	if r.StatusCode != 200 {
		r.Body.Close()
		switch r.StatusCode {
		case 404:
			return nil, info, fedora.ErrNotFound
		case 401:
			return nil, info, fedora.ErrNotAuthorized
		default:
			return nil, info, fmt.Errorf("Received status %d from bendo", r.StatusCode)
		}
	}
	info.Type = r.Header.Get("Content-Type")
	info.Length = r.Header.Get("Content-Length")
	info.Disposition = r.Header.Get("Content-Disposition")
	return r.Body, info, nil
}
