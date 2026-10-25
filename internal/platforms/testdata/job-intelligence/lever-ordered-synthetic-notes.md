# Lever ordered-content fixture notes

`lever-ordered-synthetic.json` is a constructed response for JI-021. Its ID,
site slug, title, prose, lists, location, commitment, workplace type and salary
are synthetic; no employer text or applicant data is copied into the fixture.
It intentionally includes combined and component descriptions together so the
mapper can prove it uses the combined description once, then appends named
lists in provider order and closing instructions.

## Provider evidence

- Lever's [official Postings API documentation](https://github.com/lever/postings-api/blob/master/README.md) documents global and EU instances; `GET /v0/postings/SITE/POSTING-ID`; a unique posting `id`; combined `description` / `descriptionPlain`; `opening` / `openingPlain`; body-only `descriptionBody` / `descriptionBodyPlain`; named `lists` entries with `text` and `content`; closing `additional` / `additionalPlain`; `workplaceType`; `categories.location` and `categories.commitment`; and optional `salaryRange`.
- The same official documentation describes `lists[].content` as unstyled HTML list elements and the combined `description` as opening plus body. This is why the adapter keeps the combined field authoritative when present, retains each named list once, and does not append opening/body components a second time.
- Current public global and EU hosted pages were read without opening application forms: [Employ global posting](https://jobs.lever.co/employ/a4ec1fd5-d606-4995-a745-5a8b7848937c) and [Lever EU API Engineer posting](https://jobs.eu.lever.co/lever/065f4538-7347-4207-909f-4ea68f63b4af). They confirm those present-day host/path shapes and show headings, qualifications/responsibilities, and trailing content in the rendered posting. They are UI observations, not captures of the API JSON.
- The current official README's sample `leverdemo` posting ID returned HTTP 404 when opened during this qualification. It is treated as a stale example, not live-shape evidence. Direct JSON requests for the two current postings were inaccessible through the available web fetch tool, so a current live API body and its exact content encoding remain **NOT VERIFIED** here. Tests use the documented wire fields and this explicit synthetic fixture.

## Projection and bounds

The canonical description still uses nonblank `descriptionPlain`, then
`description` converted with the existing `descriptionText`, then the existing
opening/body HTML fallback; named lists and closing text remain after it.
`descriptionBodyPlain` does not change canonical projection. If combined fields
and body HTML are absent, it is used only in the rich source representation so
reading content is preserved without changing canonical copy. The rich source
composes safe parser input in opening/body → named lists → closing order,
escapes list headings and plain fallback text, and retains the full canonical
projection as plain fallback. Optional source facts are the primary location,
raw workplace type, commitment, and existing structured salary summary. It does
not infer secondary locations, employment enum values, or remote eligibility.

The public API response is bounded by the existing posting client at 4 MiB.
The retained source is rejected as typed `too_large` if its aggregate selected
HTML/plain inputs exceed `MaxPostingSourceInputBytes` or its validated JSON
envelope exceeds `MaxPostingSourceJSONBytes` (both 1 MiB). Canonical metadata
remains available even when rich-source retention is too large.
