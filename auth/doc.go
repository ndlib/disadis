// Package auth implements authorization endpoints for disadis. While use of
// this package is not deprecated, it is discouraged. For the initial use case
// of pubtkts, auth is simple and fairly self contained. However, for most uses
// with a Hydra-based rails application, authorization is anything but simple
// due to the user identity being represented inside a rails cookie as a key
// into a database. It is somewhat difficult to extract that information and it
// requires disadis to be connected to the rails database.
//
// A useful workaround has been to use the rails application to authorize the
// download request, and then use an nginx redirect (X-Accel-Redirect) to hand
// the download off to disadis. In turn, disadis is configured to not do any
// authorization and to treat every request as permissible.
package auth
