Disadis
=======

Disadis is an authorization proxy for Hydra-based applications.
It is designed to proxy content out of Fedora,
or to verify authorization and redirect requests to other content delivery methods.
Disadis takes the burden of downloads from the hydra application.
It is designed to be fast, as well as understanding `hydraRightsmetadata` datastreams.
It can be adapted to work with different ways of indication user identity in the request.
At the moment it supports rails cookies and [auth-pubtkt](https://neon1.net/mod_auth_pubtkt/) headers.

Disadis

* supports the sufia `/downloads/` route as well as an extension which includes version information.
* provides E-tags.
* responds to `GET` and `HEAD` requests.
* assumes the filename is the label of the datastream.

Each handler can optionally use authorization or not.
This way, say, thumbnails can just be served without doing any authorization.
The handlers can each specify which datastream to proxy.

# Configuration

The code was originally written to expose each handler on a seperate port.
However, being able to directly replace sufia's `/downloads/` route was too tempting,
and so multiple handlers can be combined on a single port.
In that case the handlers are disambiguated not by path or method,
but by the `datastream_id` parameter.

The daemon takes a command line argument which names a configuration file.
The file gives how to determine the current user from a request, the handlers to set up, and the address fedora is at.

# Example

The following config file duplicates the sufia `/downloads/` handler.

```
[general]
fedora-addr = http://fedoraAdmin:fedoraAdmin@localhost:8983/fedora

[Handler "thumbnail"]
datastream = thumbnail
prefix = sufia:
port = 8081
datastream-id = thumbnail

[Handler "dl"]
auth = true
datastream = content
port = 8081
prefix = vecnet:
```

This will have disadis listen on port 8081 for connections.
The thumbnail handler proxies out the thumbnail datastream,
does not perform authentication,
and assumes any identifiers have the namespace of `sufia:`.
The `datastream-id` tells disadis to route requests having `datastream_id=thumbnail`
to this handler.
The `dl` handler serves the `content` datastream, and since it is missing
a `datastream-id` field, it is the default handler for port 8081.

Disadis assumes the requests it receives have the form `/:id` or, optionally, `/:id/:version`.
(The latter are only used if a handler has `versioned = true` in its config section).
This means requests routed to disadis need to have their initial path prefix stripped.
The following nginx config handles this.
This configuration also redirects any errors back to `@app`, so a common error page can
be displayed.

```
location ^~ /downloads/ {
    proxy_intercept_errors on;
    error_page 401 404 = @app;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $http_x_forwarded_proto;
    proxy_redirect off;
    proxy_buffering off;
    proxy_pass http://127.0.0.1:8081/;
}
```

# Future

* Range requests
* [X-Acel-Redirect](http://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_ignore_headers)
* Verify CAS tickets
* Support more than one authorization method
* Is there a simpler way to configure the whole thing? It seems too complicated to me.
* Support config reloading and graceful shutdowns
* Add metrics to track the cache hit/miss rates
