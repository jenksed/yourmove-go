# Greenhouse posting fixture

`greenhouse-encoded-posting-synthetic.json` is a constructed test fixture, not a captured employer response. Its `id`, `title`, `content`, and nested `location.name` field names and the entity-encoded markup shape follow the public Greenhouse Job Board API example at <https://docs.greenhouse.io/job-board> (read 2026-10-24). The role title, location, description, and numeric ID are synthetic.

The public API documentation states that GET endpoints expose job board data without authentication, identifies `id` as the job post identity, and documents the job-post endpoint. It shows the `content` field with HTML represented through entities and a nested location name. The fixture uses this documented shape to test decoding; it does not claim to reproduce a live API response byte-for-byte.
