# SmartRecruiters synthetic fixture

`smartrecruiters-posting-synthetic.json` is a synthetic response based on the
official Posting API `PostingDetails` shape and the bounded public Sportradar
response observed on 2026-10-24. The company, IDs, compensation, section text,
and URLs in this fixture are synthetic; they are not copied from the live job.

The fixture exercises the four documented job-ad text sections in their stable
semantic order. The optional `videos`, `postingUrl`, and `applyUrl` fields are
included to verify that URL-valued response fields are never fetched as part of
posting retrieval.
