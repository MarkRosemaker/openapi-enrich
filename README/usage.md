```bash
go get -tool github.com/MarkRosemaker/openapi-enrich/cmd/openapi-enrich
```

or

```bash
go get github.com/MarkRosemaker/openapi-enrich
```


```go
import (
    enrich "github.com/MarkRosemaker/openapi-enrich"
    "github.com/MarkRosemaker/openapi-enrich/cassette"
)

// Start from a minimal document or load an existing spec.
doc := enrich.NewDocument()

interactions := []cassette.Interaction{
    {
        Request: cassette.Request{
            Method:  "GET",
            URL:     "https://api.example.com/users",
            Headers: http.Header{},
        },
        Response: cassette.Response{
            StatusCode: http.StatusOK,
            Headers:    http.Header{"Content-Type": {"application/json"}},
            Body:       []byte(`[{"id":1,"name":"Alice"}]`),
        },
    },
}

if err := enrich.Enrich(doc, interactions); err != nil {
    log.Fatal(err)
}
```

The main public function is:

```go
func Enrich(doc *openapi.Document, interactions cassette.Interactions) error
```

Schemas are left inline — the caller composes any post-processing as needed.
