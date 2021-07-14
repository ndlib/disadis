package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ndlib/disadis/fedora"
)

// DownloadHandler handles the routes
//
//	GET	/:id
//	HEAD	/:id
//      GET    /:id/zip/id1,id2,id3
//
//
// The first routes will return the contents of the
// datastream named Ds.
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
//	http.Handle("/d/", http.StripPrefix("/d/", dh))
//	return http.ListenAndServe(":"+port, nil)
type DownloadHandler struct {
	Fedora     fedora.Fedora // connection to fedora
	Ds         string        // the datastream to proxy
	Prefix     string        // the PID prefix to use, needs colon
	BendoToken string        // optional, used for 'E' and 'R' datastreams
}

// The generic HTTP handler - parses the routes
// and calls the route-specific sub-handlers

func (dh *DownloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	path = strings.TrimSuffix(path, "/")
	// should always return a string of length 1 or 3
	components := strings.SplitN(path, "/", 3)

	// will an identifier ever have more than 64 characters?
	if len(components[0]) == 0 || len(components[0]) > 64 {
		http.NotFound(w, r)
		return
	}

	pid := dh.Prefix + components[0] // sanitize pid somehow?

	//Valid routes are /:id (single file download)
	//and /:id/zip/:id1,:id2,...idn (zip of all files associated with :id
	//return MethodNotAllowed for others
	switch {
	case len(components) == 1:
		dh.downloadSingleFile(pid, w, r)
	case len(components) == 3 && components[1] == "zip":
		dh.downloadZip(pid, w, r, components[2])
	default:
		http.NotFound(w, r)
	}
}

// private method that downloads content for given pid.
// works with both inline content in fedora, or indirect content from bendo
func (dh *DownloadHandler) downloadSingleFile(pid string, w http.ResponseWriter, r *http.Request) {
	// always hit fedora for most recent info
	// Should this lookup be cached?
	dsinfo, err := dh.Fedora.GetDatastreamInfo(pid, dh.Ds)
	if err != nil {
		log.Printf("Received Fedora error (%s,%s): %s", pid, dh.Ds, err.Error())
		http.NotFound(w, r)
		return
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
	var content io.ReadSeeker
	var info fedora.ContentInfo
	if dh.BendoToken != "" && dsinfo.LocationType == "URL" {
		// this datastream is stored outside of fedora
		// Get the content directly. This way we can supply the auth headers
		// directly to the content supplier.
		content, info, err = getBendoContent(dsinfo.Location, dh.BendoToken)
	} else {
		// get the content from fedora
		// use StreamSeeker to handle range requests.
		var data io.ReadCloser
		data, info, err = dh.Fedora.GetDatastream(pid, dh.Ds)
		n, _ := strconv.ParseInt(info.Length, 10, 64)
		content = NewStreamSeeker(data, n)
		defer data.Close()
	}
	if err != nil {
		switch err {
		case fedora.ErrNotFound:
			http.NotFound(w, r)
			return
		default:
			log.Println("Received error:", err)
			http.Error(w, "500 Internal Error", http.StatusInternalServerError)
			return
		}
	}

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
	if info.MD5 == "" && dsinfo.Checksum != "" {
		// If we did not get a checksum from the content supplier,
		// use the MD5 checksum in the fedora metadata, if any
		info.MD5 = dsinfo.Checksum
	}
	if info.MD5 != "" {
		w.Header().Set("Content-Md5", info.MD5)
	}
	if info.SHA256 != "" {
		w.Header().Set("Content-Sha256", info.SHA256)
	}

	// Use the size returned from the content request in case we redirected
	n, _ := strconv.ParseInt(info.Length, 10, 64)
	// Don't support or use range requests if we either
	//  1) Don't know the content length, or
	//  2) Are downloading an PDF.
	//
	// The latter condition is to work around a bug with the internal PDF
	// viewer in Chrome that doesn't send cookies for range requests coupled
	// with the desire of the viewer to download PDFs in sections using range
	// requests. This causes auth failures for private or nd-only files. When
	// the bug is fixed this workaround can be removed.
	//
	// See https://bugs.chromium.org/p/chromium/issues/detail?id=961617
	if n <= 0 || dsinfo.MIMEType == "application/pdf" {
		if n > 0 {
			w.Header().Set("Content-Length", info.Length)
		}
		if r.Method == "HEAD" {
			return
		}
		// Since we are not supporting range requests, the only thing to do is
		// copy the file out.
		_, err = io.Copy(w, content)
		if err != nil {
			log.Println(err)
		}
		return
	}

	// use ServeContent to handle range requests.
	http.ServeContent(w, r, dsinfo.Label, time.Time{}, content)
}

// downloadZip streams a zip file that contains the contents of the files
// identified in the pidlist.
//
// assuming route /:pid1/zip/:pid2,:pid3..n
// return zip file named pid1.zip containing files for pid1 , pid2, ...pid3
// Now that we are actually streaming the zipfile back to the http responsewriter
// as it is being written, to avoid having to buffer a large file on the local disadis machine
func (dh *DownloadHandler) downloadZip(pid string, w http.ResponseWriter, r *http.Request, pidlist string) {

	// For the time being, nosupport of HEAD requests
	if r.Method == "HEAD" {
		http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// expect  a list of pids
	pids := strings.Split(pidlist, ",")

	// open the zip file stream- write straight the httpResponseWriter

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	w.Header().Set("Content-Disposition", `inline; filename="`+pid+`.zip"`)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Transfer-Encoding", "binary")
	w.Header().Set("Cache-Control", "private")

	// for each pid in list
	// retrieved content from fedora or bendo
	// write to zip stream
	for _, this_pid := range pids {
		// Get Fedora Info
		dsinfo, err := dh.Fedora.GetDatastreamInfo(dh.Prefix+this_pid, dh.Ds)
		if err != nil {
			log.Printf("Received Fedora error (%s,%s): %s", this_pid, dh.Ds, err.Error())
			continue
		}

		// return content
		var content io.ReadCloser

		if dh.BendoToken != "" && dsinfo.LocationType == "URL" {
			// this datastream is stored outside of fedora
			// Get the content directly. This way we can supply the auth headers
			// directly to the content supplier.
			content, _, err = getBendoContent(dsinfo.Location, dh.BendoToken)
		} else {
			// get the content from fedora
			content, _, err = dh.Fedora.GetDatastream(dh.Prefix+this_pid, dh.Ds)
		}
		if err != nil {
			switch err {
			case fedora.ErrNotFound:
				log.Printf("Content not found (zip:%s/%s)", pid, this_pid)
				continue
			default:
				log.Printf("Received fedora error (zip:%s/%s): %s", pid, this_pid, err)
				continue
			}
		}

		header := zip.FileHeader{
			Name:     dsinfo.Label,
			Method:   zip.Deflate,
			Modified: time.Now(), // can we get a modified time for the file somehow?
			Comment:  "CurateND:" + this_pid,
		}
		zip_filep, err := zipWriter.CreateHeader(&header)
		if err != nil {
			log.Printf("zip:%s/%s: %s", pid, this_pid, err)
			content.Close()
			continue
		}
		// Stream the file conetent from the content ReadCloser to the ZipFile Writer
		_, err = io.Copy(zip_filep, content)
		content.Close()
		if err != nil {
			log.Printf("io.Copy: zip:%s/%s: %s", pid, this_pid, err)
			return // a copy error is most likely a broken pipe.
		}
	}
	zipWriter.SetComment("Downloaded from CurateND: " + pid)
}

// returns the contents of the given URL
// The returned stream needs to be closed when finished.
func getBendoContent(url, token string) (io.ReadSeekCloser, fedora.ContentInfo, error) {
	var info fedora.ContentInfo
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, info, err
	}
	req.Header.Add("X-Api-Key", token)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, info, err
	}
	r.Body.Close()
	if r.StatusCode != 200 {
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
	info.MD5 = r.Header.Get("X-Content-Md5")
	info.SHA256 = r.Header.Get("X-Content-Sha256")
	length, _ := strconv.ParseInt(info.Length, 10, 64)
	return NewHTTPSeeker(url, length, token), info, nil
}
