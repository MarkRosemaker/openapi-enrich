# Roadmap

Work not yet done. An entry is deleted once it is.

## Keep a shared component from being narrowed by one operation's traffic

`testdata/go-pkgsite/openapi.json` used to share one `PaginatedResponse`
between six places whose items differ (`SearchResult`, `ModuleVersion`,
`Vulnerability`, …), with `items` left as `{"type": "object"}`. The
recording only pages through `/versions`, so enriching filled the shared
items with module-version properties, and after openapi-flatten and
openapi-compress the specification claimed that `/search` and `/vulns`
return lists of `ModuleVersion`. The input now gives each endpoint its own
response schema, typed by its items as pkgsite's `PaginatedResponse[T]` is.

What is left is a decision: whether enriching should refuse to add
properties to a component that several operations share when the evidence
comes from only one of them. That is a heuristic that changes behaviour for
every specification, so it is the owner's call.
