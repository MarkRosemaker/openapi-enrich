What it infers:

- **Paths** — detected from request URLs, with ID-like segments replaced by
  `{param}` path parameters.
- **Operations** — one per unique method + path, with an inferred `operationId`
  (e.g. `GET /users` → `ListUsers`, `GET /users/{id}` → `GetUserByID`).
- **Query parameters** — schema inferred from values; comma-separated values
  become non-exploded arrays.
- **Request headers** — `Authorization` creates an HTTP security scheme;
  `x-*` and other custom headers become header parameters.
- **Security** — an operation only called without `Authorization` gets
  `security: []`, and one called both with and without it gets the credential
  as optional (`{}` beside the scheme). A security list every operation shares
  is stated once at the document level.
- **Request bodies** — JSON bodies produce inline object schemas.
- **Responses** — JSON, text/plain, and text/html responses are modeled;
  repeated observations are merged.
- **Arrays of objects** — the elements of a recorded array of objects meet the
  specification one by one, so in a list of mixed variants, such as Notion's
  blocks, each reaches the variant of a union it matches rather than all of them
  one. With no union there, they merge into one item as before.
- **Binary bodies** — a request or response body that is not text, such as a
  zip, a PDF, an image or a video, is documented by its media type as a string
  of bytes (`{"type": "string", "format": "binary"}`), from its headers alone.
- **Schema formats** — UUID, URI, email, date-time, IPv4, IPv6 are detected
  automatically from string values.
- **Nulls and empty arrays** — a value only ever seen as `null` has the type
  `null`, and becomes nullable once it is seen with a real type
  (`["string", "null"]`). An array only ever seen empty is
  `{"type": "array", "maxItems": 0}`, until a non-empty one shows its items.
- **Schema types** — a schema in the given document that has no `type` gets
  the one its `enum` or `const` values share, e.g. `{"const": 401}` becomes
  an `integer`; a `null` among them makes it nullable.
- **Enums** — an `enum` already declared in the given document grows with
  every value observed for it. An object's keys count too, when its
  `propertyNames` declares an enum. A recording never starts an enum of its own.
- **Shared components** — a schema several operations refer to is documented
  from whichever of them was recorded, since sharing says they have the same
  shape. Recording the others widens it to fit all of them. Where the sharing
  itself is wrong, give each operation its own schema in the input.

The module also ships the pieces needed to *obtain* that traffic:

- `cassette` — self-contained HTTP interaction types, with JSON persistence,
  bearer-token masking, and header trimming before anything is written to disk.
  A body that is not text is never read or written, only marked `bodyOmitted`,
  so a large download streams to its caller as it is. A JSON string longer than
  2,048 bytes, such as an image in base64, is cut to that and ends in `…`.
- `recorder` — an `http.RoundTripper` that records live traffic into a cassette,
  so you can capture interactions by pointing an existing client at it.
