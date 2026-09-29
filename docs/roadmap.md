# Roadmap

Work not yet done. An entry is deleted once it is.

## Keep a shared component from being narrowed by one operation's traffic

In `testdata/go-pkgsite/openapi.json`, `PaginatedResponse` is one
component shared by six places: the `/search`, `/versions/{path}` and
`/vulns/{path}` responses and three properties. Its `items` is
`{"type": "array", "items": {"type": "object"}}`, so it serves as a
generic wrapper whose items differ by endpoint (`SearchResult`,
`ModuleVersion`, `Vulnerability`, …). The recording only pages through
`/versions`, so enriching fills the shared items with module-version
properties. openapi-flatten names that schema
`PaginatedResponseItemsItem`, and openapi-compress rightly finds it
identical to `ModuleVersion` and merges them. The result claims that
`/search` and `/vulns` return lists of `ModuleVersion`. Validation still
passes, since nothing is required and other properties stay allowed, but
generated code would decode those responses into the wrong type.

Two things to do:

- Fix the input: give each endpoint its own response schema whose
  `items` names its type, sharing `nextPageToken` and `total` through
  `allOf` or by repeating them. `$dynamicRef` would be JSON Schema's own
  way to write the generic wrapper, but the openapi library does not
  model it yet.
- Decide whether enriching should refuse to add properties to a
  component that several operations share when the evidence comes from
  only one of them. That is a heuristic that changes behaviour for every
  specification, so it is the owner's call.
