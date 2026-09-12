---
tagline: Build an API spec out of the traffic you already have.
logo:
    alt: A gopher watching envelopes fly past through binoculars while sketching in a notebook
    source: openapi-enrich.jpg
    width: 500
---

<div align="center" id=badges>

![Code Coverage](https://img.shields.io/badge/coverage-72.4%25-yellowgreen)

</div>





`openapi-enrich` enriches an [OpenAPI 3.1](https://spec.openapis.org/oas/v3.1.0)
document from observed HTTP traffic. Feed it a document and a set of recorded
request/response pairs, and it adds the paths, operations, parameters, request
bodies, and response schemas it can infer from them.
