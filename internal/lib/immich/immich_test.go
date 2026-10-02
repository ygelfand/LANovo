package immich

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func fake(t *testing.T, seen *map[string]any) *Client {
	mux := http.NewServeMux()
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("x-api-key") != "k3y" {
				http.Error(w, `{"message":"Invalid API key"}`, http.StatusUnauthorized)
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /api/albums", auth(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"id":"a1","albumName":"Kitchen","assetCount":3},{"id":"a2","albumName":"Trips"}]`))
	}))
	mux.HandleFunc("GET /api/tags", auth(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"id":"t1","name":"display","value":"home/display"},{"id":"t2","name":"cats","value":"cats"}]`))
	}))
	mux.HandleFunc("POST /api/search/random", auth(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(seen)
		w.Write([]byte(`[{"id":"p1","type":"IMAGE"},{"id":"p2","type":"IMAGE"}]`))
	}))
	mux.HandleFunc("GET /api/assets/{id}/thumbnail", auth(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("size") != "preview" {
			http.Error(w, "size", 400)
			return
		}
		w.Write([]byte("picture of " + r.PathValue("id")))
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return New(srv.URL+"/", "k3y")
}

func TestItResolvesNamesAndAsksForRandomImages(t *testing.T) {
	var seen map[string]any
	c := fake(t, &seen)
	ctx := context.Background()

	albums, tags, err := c.Resolve(ctx, []string{"kitchen"}, []string{"home/display", "Cats"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(albums, []string{"a1"}) || !reflect.DeepEqual(tags, []string{"t1", "t2"}) {
		t.Fatalf("albums %v tags %v", albums, tags)
	}
	got, err := c.Random(ctx, albums, tags, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "p1" {
		t.Errorf("assets %+v", got)
	}
	if seen["type"] != "IMAGE" || seen["size"] != float64(5) || !reflect.DeepEqual(seen["albumIds"], []any{"a1"}) || !reflect.DeepEqual(seen["tagIds"], []any{"t1", "t2"}) {
		t.Errorf("search body %v", seen)
	}
	b, err := c.Preview(ctx, "p2")
	if err != nil || string(b) != "picture of p2" {
		t.Errorf("preview %q %v", b, err)
	}
}

func TestNoFilterSendsNoIDs(t *testing.T) {
	var seen map[string]any
	c := fake(t, &seen)
	if _, err := c.Random(context.Background(), nil, nil, 1); err != nil {
		t.Fatal(err)
	}
	if _, ok := seen["albumIds"]; ok {
		t.Errorf("albumIds sent: %v", seen)
	}
	if _, ok := seen["tagIds"]; ok {
		t.Errorf("tagIds sent: %v", seen)
	}
}

func TestFailuresSayWhat(t *testing.T) {
	var seen map[string]any
	c := fake(t, &seen)
	ctx := context.Background()
	if _, _, err := c.Resolve(ctx, []string{"Nope"}, nil); err == nil {
		t.Error("an unknown album resolved")
	}
	if _, _, err := c.Resolve(ctx, nil, []string{"nope"}); err == nil {
		t.Error("an unknown tag resolved")
	}
	c.Key = "wrong"
	if _, err := c.Albums(ctx); err == nil {
		t.Error("a wrong key was accepted")
	}
	if _, err := New("", "k").Albums(ctx); err == nil {
		t.Error("no server was accepted")
	}
}
