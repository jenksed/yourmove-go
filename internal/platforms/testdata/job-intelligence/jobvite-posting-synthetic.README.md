# Jobvite synthetic fixture

`jobvite-posting-synthetic.html` is synthetic test data. Its JSON-LD shape was
derived from a read-only capture of the public detail page
`https://jobs.jobvite.com/egnyte/job/oVqQAfwv?nl=1`. The captured response was
80,609 bytes (SHA-256
`759ffa1f00c498ae78e120793748a217e3404ef370016e9de00bf3f1dd6cf5f7`). No
employer description, identifiers, organization name, or location values from
that response are included in this fixture.

The fixture has one `JobPosting` JSON-LD record, an opaque string identifier,
an optional matching detail URL, an HTML description with ordered headings and
lists, explicit organization/employment/location facts, and a salary shape
whose quoted amounts fail the existing numeric salary validator. The URL,
identifier, text, location, and all other values are invented for tests.
