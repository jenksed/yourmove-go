# Workable public widget fixture

`workable-widget-synthetic.json` is a synthetic fixture based on the public
Workable widget response shape observed in the read-only sample
`/private/tmp/yourmove-go-competitive-start-20261024/live-sources/workable_widget.body`
(SHA-256 `38a31b0d6911dc73d87b84dea53bf893922a5c97d1ba0367405de3f70e251caa`).
The observed response had an account name, a `jobs` array with `shortcode`,
title, description HTML, employment type, telecommuting, location fields, and
an ordered `locations` array. The fixture replaces the employer and posting
content with synthetic Northstar data and includes one nonmatching job to
verify exact shortcode selection. Returned URL fields are decoys and must not
be fetched.
