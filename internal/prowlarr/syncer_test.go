package prowlarr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vavallee/bindery/internal/models"
)

func TestIndexerTypeForProtocol(t *testing.T) {
	cases := []struct {
		protocol string
		want     string
	}{
		{"usenet", "newznab"},
		{"torrent", "torznab"},
		{"", "torznab"},
		{"anything-else", "torznab"},
	}
	for _, c := range cases {
		if got := indexerTypeForProtocol(c.protocol); got != c.want {
			t.Errorf("indexerTypeForProtocol(%q) = %q, want %q", c.protocol, got, c.want)
		}
	}
}

// fakeIndexerStore records calls so tests can assert which path the syncer took.
type fakeIndexerStore struct {
	existing []models.Indexer
	created  []models.Indexer
	updated  []models.Indexer
	deleted  []int64
}

func (f *fakeIndexerStore) ListByProwlarrInstance(context.Context, int64) ([]models.Indexer, error) {
	return f.existing, nil
}

func (f *fakeIndexerStore) Create(_ context.Context, idx *models.Indexer) error {
	f.created = append(f.created, *idx)
	return nil
}

func (f *fakeIndexerStore) Update(_ context.Context, idx *models.Indexer) error {
	f.updated = append(f.updated, *idx)
	return nil
}

func (f *fakeIndexerStore) Delete(_ context.Context, id int64) error {
	f.deleted = append(f.deleted, id)
	return nil
}

type fakeInstanceStore struct{}

func (fakeInstanceStore) SetLastSyncAt(context.Context, int64, time.Time) error { return nil }

func prowlarrStub(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/indexer" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

// Newly-discovered indexers are created with the Type derived from the
// Prowlarr protocol — "usenet" -> "newznab", everything else -> "torznab".
func TestSync_CreatesIndexerWithTypeFromProtocol(t *testing.T) {
	srv := prowlarrStub(t, `[
		{"id":1,"name":"NZB One","protocol":"usenet","supportsSearch":true,"categories":[{"id":7020}]},
		{"id":2,"name":"Tor One","protocol":"torrent","supportsSearch":true,"categories":[{"id":7020}]}
	]`)
	defer srv.Close()

	store := &fakeIndexerStore{}
	syncer := NewSyncer(New(srv.URL, "k"), store, fakeInstanceStore{})

	res, err := syncer.Sync(context.Background(), 42)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Added != 2 || res.Updated != 0 || res.Removed != 0 {
		t.Fatalf("result = %+v, want Added=2", res)
	}

	byName := map[string]models.Indexer{}
	for _, idx := range store.created {
		byName[idx.Name] = idx
	}
	if got := byName["NZB One"].Type; got != "newznab" {
		t.Errorf("usenet indexer Type = %q, want %q", got, "newznab")
	}
	if got := byName["Tor One"].Type; got != "torznab" {
		t.Errorf("torrent indexer Type = %q, want %q", got, "torznab")
	}
}

// An existing indexer that was mis-typed as "torznab" (by an older sync
// that hardcoded the type) must be corrected to "newznab" when Prowlarr
// now reports the protocol as "usenet". Without this, NZB releases keep
// routing to qBittorrent and fail with "hash could not be determined".
func TestSync_CorrectsMisTypedExistingIndexer(t *testing.T) {
	srv := prowlarrStub(t, `[
		{"id":1,"name":"NZB One","protocol":"usenet","supportsSearch":true,"categories":[{"id":7020}]}
	]`)
	defer srv.Close()

	pID := 1
	instID := int64(42)
	store := &fakeIndexerStore{
		existing: []models.Indexer{{
			ID:                 100,
			Name:               "NZB One",
			Type:               "torznab",
			URL:                srv.URL + "/1/api",
			Categories:         []int{7020},
			Enabled:            true,
			ProwlarrInstanceID: &instID,
			ProwlarrIndexerID:  &pID,
		}},
	}
	syncer := NewSyncer(New(srv.URL, "k"), store, fakeInstanceStore{})

	res, err := syncer.Sync(context.Background(), instID)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Updated != 1 {
		t.Fatalf("result = %+v, want Updated=1", res)
	}
	if len(store.updated) != 1 || store.updated[0].Type != "newznab" {
		t.Fatalf("updated indexer = %+v, want Type=newznab", store.updated)
	}
}

// An existing indexer whose Type already matches and whose other fields are
// unchanged should NOT trigger an Update — the Update path is reserved for
// meaningful changes so sync traffic stays low.
func TestSync_NoUpdateWhenUnchanged(t *testing.T) {
	srv := prowlarrStub(t, `[
		{"id":1,"name":"NZB One","protocol":"usenet","supportsSearch":true,"categories":[{"id":7020}]}
	]`)
	defer srv.Close()

	pID := 1
	instID := int64(42)
	store := &fakeIndexerStore{
		existing: []models.Indexer{{
			ID:                 100,
			Name:               "NZB One",
			Type:               "newznab",
			URL:                srv.URL + "/1/api",
			Categories:         []int{7020},
			Enabled:            true,
			ProwlarrInstanceID: &instID,
			ProwlarrIndexerID:  &pID,
		}},
	}
	syncer := NewSyncer(New(srv.URL, "k"), store, fakeInstanceStore{})

	res, err := syncer.Sync(context.Background(), instID)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Updated != 0 || len(store.updated) != 0 {
		t.Fatalf("result = %+v, updates = %d, want no updates", res, len(store.updated))
	}
}
