What it infers:

- **Paths** — detected from request URLs, with ID-like segments replaced by
  `{param}` path parameters.
- **Operations** — one per unique method + path, with an inferred `operationId`
  (e.g. `GET /users` → `ListUsers`, `GET /users/{id}` → `GetUserByID`).
- **Query parameters** — schema inferred from values; comma-separated values
  become non-exploded arrays.
- **Request headers** — `Authorization` creates an HTTP security scheme;
  `x-*` and other custom headers become header parameters.
- **Request bodies** — JSON bodies produce inline object schemas.
- **Responses** — JSON, text/plain, and text/html responses are modeled;
  repeated observations are merged.
- **Schema formats** — UUID, URI, email, date-time, IPv4, IPv6 are detected
  automatically from string values.

The module also ships the pieces needed to *obtain* that traffic:

- `cassette` — self-contained HTTP interaction types, with JSON persistence,
  bearer-token masking, and header trimming before anything is written to disk.
- `recorder` — an `http.RoundTripper` that records live traffic into a cassette,
  so you can capture interactions by pointing an existing client at it.
