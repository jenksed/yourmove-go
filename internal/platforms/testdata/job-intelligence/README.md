# Job Intelligence provider characterization fixture

`ashby-long-synthetic.json` is a synthetic Ashby posting-board response used to characterize the current Go mapper. It contains no copied employer prose, candidate material, or real posting identifier. The UUID, board slug, title, description, URL values, location, enums, and salary values are invented for this test. Its HTML/plain description content intentionally exercises headings, paragraphs, nested unordered and ordered lists, emphasis context, a link, direct posting facts, and compensation.

The outer wire shape and documented field names follow Ashby's public Job Postings API response (`apiVersion`, `jobs[]`, title, description variants, location/workplace/employment fields, job/apply URLs, and compensation): <https://developers.ashbyhq.com/docs/public-job-posting-api>. The dated M0 source note records prior read-only Ashby and Linear public-board observations on 2026-10-24, including both description variants, these direct-fact families, `jobs[].compensation`, and job/apply URL patterns. Those public responses had SHA-256 identities recorded in `docs/implementation/job-intelligence/IMPLEMENTATION_PLAN.md`; this fixture is not byte-derived from either response.

## Current mapper behavior characterized

- `descriptionPlain` wins after `strings.TrimSpace`; internal spacing and line breaks remain byte-for-byte as supplied.
- If plain text is blank, `descriptionHtml` is converted by the existing `descriptionText` tokenizer. This is flattened text, not structured HTML preservation.
- A nonempty `jobs[].compensation.scrapeableCompensationSalarySummary` wins unchanged and is labeled `employer`; when absent, the existing description salary parser can populate the summary and labels it `description`.
- The current mapper returns title, description, and salary metadata only. The direct location/workplace/employment fields and `applyUrl` in this fixture are ignored by `extractAshbyMetadata`.

## Identity matrix status

These are synthetic characterization cases. A result marked “currently accepted” describes existing behavior only and is a known gap to resolve in the separately authorized M1 identity task; it is not a safe-source contract.

| Case | Current result | Evidence class |
|---|---|---|
| Saved URL parser receives exact HTTPS Ashby board / UUID path | Accepts and returns path board plus UUID | Synthetic URL; parser behavior tested |
| Present `jobs[].id` with no `jobUrl` | Mapper currently accepts by ID, but this is not a source-qualified positive; board-context evidence is still required | Synthetic current-behavior characterization only. The official field table omits `jobs[].id`; all 101 dated sample records having IDs does not prove an ID-only shape |
| Missing `jobs[].id` with `jobUrl` on the requested board and matching UUID suffix | Mapper accepts using `jobUrl` suffix | Constructed positive; not observed absent ID in the 101 dated jobs |
| Same UUID in `id`, but `jobUrl` names another Ashby board | Mapper currently accepts by `id`; board is not an argument to the mapper | Constructed negative; current unsafe acceptance tested |
| Missing ID and same UUID suffix in `jobUrl` on another Ashby board | Mapper currently accepts by host and UUID suffix; it does not compare board | Constructed negative; current unsafe acceptance tested |
| Missing ID and matching UUID suffix in an `http://jobs.ashbyhq.com/...` `jobUrl` | Mapper currently accepts because URL fallback checks hostname/path, not HTTPS | Constructed negative; current unsafe acceptance tested. Userinfo and explicit-port variants are NOT VERIFIED by fixture |
| `id` conflicts with a matching selected UUID suffix in `jobUrl` | Mapper currently accepts via URL fallback | Constructed negative; current unsafe acceptance tested |
| Selected `id` with a foreign or malformed `jobUrl` | Mapper currently accepts by `id` before examining the URL | Constructed negative; current unsafe acceptance tested |
| Two records with the same selected ID | Mapper silently returns the first matching record | Constructed ambiguity; current first-match behavior tested; JI-005 must reject ambiguous selection |
| Selected `id` plus an unsafe/foreign `applyUrl` | Mapper ignores `applyUrl` and returns metadata | Constructed destination negative; no destination validation occurs in this mapper |
| Missing ID with a different posting UUID suffix in `jobUrl` | Mapper does not match by that URL; current not-found error wraps `ErrClosed` | Constructed negative; does not prove actual closure |
| Missing ID with a same-UUID non-Ashby `jobUrl` | URL fallback rejects the wrong hostname; current not-found error wraps `ErrClosed` | Constructed negative; does not prove actual closure |

Synthetic malformed JSON and a non-array `jobs` value are also characterized as decode errors, not affirmative closure evidence.

`parseAshbyURL` checks HTTPS, exact `jobs.ashbyhq.com` hostname, absence of credentials and explicit ports, a two-segment path, a safe board token, and a full UUID. It does not qualify the board against a posting response and allows query/fragment data. This syntax behavior is distinct from selected-record identity and application-destination policy. Public application-schema inspection owns its own destination checks; these posting characterization tests do not grant browser or transfer authority.

## Measured sizes

At this fixture revision: the serialized synthetic response is 4,576 UTF-8 bytes; `descriptionHtml` is 2,063 bytes; `descriptionPlain` is 1,848 bytes; the description pair is 3,911 bytes. A **non-contractual** candidate selected-source JSON projection measures 4,980 bytes when encoded with Go `encoding/json` (default HTML escaping). It uses version `1`, provider/board/posting identity, HTML/plain pair, canonical-description SHA-256, a 64-zero source-digest placeholder, primary location, workplace type, employment type, and salary summary. The measurement can be reproduced from this fixture with the exact map keys and values below; `encoding/json` sorts map keys. This is a sizing projection only; it does not define a Go type or JSON contract, and JI-002 owns the final source type/digest projection.

```go
canonical := strings.TrimSpace(job.DescriptionPlain)
canonicalHash := sha256.Sum256([]byte(canonical))
projection := map[string]any{
    "version": 1,
    "identity": map[string]string{
        "provider": "ashby", "board": "fable",
        "posting_id": job.ID,
    },
    "description_html": job.DescriptionHTML,
    "description_plain": job.DescriptionPlain,
    "canonical_description_sha256": hex.EncodeToString(canonicalHash[:]),
    "source_content_sha256": strings.Repeat("0", 64),
    "location": job.Location,
    "workplace_type": job.WorkplaceType,
    "employment_type": job.EmploymentType,
    "salary_summary": job.Compensation.SalarySummary,
}
encoded, _ := json.Marshal(projection)
len(encoded) // 4,980
```

Here `job` is `jobs[0]` decoded from the fixture; `DescriptionHTML`, `DescriptionPlain`, `Location`, `WorkplaceType`, `EmploymentType`, and nested `Compensation.SalarySummary` map directly to the JSON keys with those names. With this projection and Go's standard serializer, its SHA-256 is `f075815d1177b516d7d6d57433cb2b3b7e7cfef6e60067cc583a411730796d8d`.
