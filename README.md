Dis-a-dis
=========

Disadis is an authorization proxy for Hydra-based applications.
It is designed to proxy content out of Fedora,
or to verify authorization and redirect requests to other content delivery methods.
Disadis takes the burden of downloads from the hydra app.
It is designed to be fast, as well as understanding `hydraRightsmetadata` datastreams.
It can be adapted to work with different ways of indication user identity in the request.
At the moment it supports rails cookies and auth-pubtkt headers.

It supports the sufia `/downloads/` route as well as an extension which includes version information.
It provides E-tags.
It responds to `GET` and `HEAD` requests.
It assumes the filename is the label of the datastream.

Each handler can either use authorization or not.
This way, say, thumbnails can just be served without doing any authorization.
The handlers can each specify which datastream to proxy.

# Example


# Future

* Range requests
* [X-Acel-Redirect](http://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_ignore_headers)
* Verify CAS tickets
* Support more than one authorization method
