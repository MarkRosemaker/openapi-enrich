package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/MarkRosemaker/openapi"
	enrich "github.com/MarkRosemaker/openapi-enrich"
	"github.com/MarkRosemaker/openapi-enrich/cassette"
	"github.com/MarkRosemaker/openapi-enrich/recorder"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dir := filepath.Join("testdata", entry.Name())

		path := filepath.Join(dir, "interactions.json")
		ias, err := cassette.InteractionsReadFile(path)
		if err != nil {
			return err
		}

		tr := recorder.NewTransport(nil, ias)

		// Call requests that don't have a response yet
		for _, ia := range ias {
			if ia.Response.StatusCode > 0 {
				continue
			}

			req, err := ia.Request.Create(ctx)
			if err != nil {
				return err
			}

			if _, err := tr.RoundTrip(req); err != nil {
				return err
			}
		}

		ias = tr.Interactions

		// These fixtures are recordings of real accounts, so redact credentials
		// and personal data before anything reaches disk. These keys are far too
		// generic for a library default to redact outright, but carry account
		// identifiers here, so they go through IDKeys: an account UUID under
		// "id" is redacted, a structural name under the same key is not.
		m := cassette.DefaultMasker()
		m.IDKeys = []string{"id", "_id", "ownerId"}
		m.NameKeys = []string{"user", "displayName", "fullName"}
		m.UsernameKeys = []string{"username", "lowerCaseUsername", "handle", "login"}
		ias.MaskWith(m)

		// regenerate interactions
		if err := ias.WriteFile(path); err != nil {
			return err
		}

		doc, err := openapi.LoadFromFile(filepath.Join(dir, "openapi.json"))
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("loading initial spec: %w", err)
			}

			doc = enrich.NewDocument()
		}

		if err := enrich.Enrich(doc, ias); err != nil {
			return fmt.Errorf("enriching %s: %w", entry.Name(), err)
		}

		// Sort responses and components (but not paths to keep the order)
		for _, path := range doc.Paths {
			for _, op := range path.Operations {
				op.Responses.Sort()
			}
		}
		doc.Components.SortMaps()

		if err := doc.WriteToFile(filepath.Join(dir, "golden.json")); err != nil {
			return err
		}
	}

	return nil
}
