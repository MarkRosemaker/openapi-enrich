package enrich

import (
	"net/http"
	"testing"

	"github.com/MarkRosemaker/openapi"
)

func TestInferOperationID(t *testing.T) {
	tests := []struct {
		method string
		path   openapi.Path
		want   string
	}{
		{http.MethodGet, "/users", "ListUsers"},
		{http.MethodGet, "/users/{id}", "GetUserByID"},
		{http.MethodGet, "/users/{userId}/posts", "ListUserPosts"},
		{http.MethodGet, "/users/{userId}/posts/{postId}", "GetUserPostByPostID"},
		{http.MethodPost, "/users", "PostUsers"},
		{http.MethodPut, "/users/{id}", "PutUserByID"},
		{http.MethodPatch, "/users/{id}", "PatchUserByID"},
		{http.MethodDelete, "/users/{id}", "DeleteUserByID"},
		{http.MethodGet, "/health", "ListHealth"},
		{http.MethodPost, "/auth/login", "PostAuthLogin"},
		{http.MethodHead, "/users", "HeadUsers"},
		{http.MethodOptions, "/users", "OptionsUsers"},
		{"CONNECT", "/proxy", "ConnectProxy"},
		{http.MethodGet, "/companyfacts/CIK{cik}.json", "ListCompanyfactsCikCikJSON"},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+string(tc.path), func(t *testing.T) {
			got := inferOperationID(tc.method, tc.path)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
