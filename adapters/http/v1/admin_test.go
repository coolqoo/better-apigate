package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/coolqoo/better-apigate/adapters/postgres"
	"github.com/coolqoo/better-apigate/domain/portal"
	"github.com/coolqoo/better-apigate/domain/wallet"
	"github.com/google/uuid"
)

func adminTestServer(t *testing.T) *Server {
	t.Helper()
	dsn := os.Getenv("APIGATE_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set APIGATE_TEST_DATABASE_DSN to run PostgreSQL admin tests")
	}
	admin, err := postgres.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	schema := "admin_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Error(err)
		}
	})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, err := postgres.Open(u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO users(id,email,role) VALUES('admin','admin@example.com','admin')"); err != nil {
		t.Fatal(err)
	}
	s, err := New(Deps{DB: db, Wallet: postgres.NewWalletStore(db), PublicURL: "http://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func adminTestGet(t *testing.T, s *Server, path, userID string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if userID != "" {
		login := httptest.NewRecorder()
		if err := s.startSession(login, r, userID); err != nil {
			t.Fatal(err)
		}
		r.AddCookie(login.Result().Cookies()[0])
	}
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)
	return w
}

type adminTestCollection[T any] struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes T      `json:"attributes"`
	} `json:"data"`
	Meta  portal.Pagination `json:"meta"`
	Links map[string]string `json:"links"`
}

func adminTestDecode[T any](t *testing.T, w *httptest.ResponseRecorder) adminTestCollection[T] {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, response = %s", w.Code, w.Body.String())
	}
	var doc adminTestCollection[T]
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestCustomersSorting(t *testing.T) {
	s := adminTestServer(t)
	for _, customer := range []struct {
		id, name, createdAt string
		balance             int64 // -1 means no wallet.
	}{
		{"a", "Match Alice", "2026-01-01T00:00:00Z", 9_000_001},
		{"b", "Match Bob", "2026-02-01T00:00:00Z", 10_000_000},
		{"c", "Match Carol", "2026-02-01T00:00:00Z", 10_000_000},
		{"d", "Dana", "2026-03-01T00:00:00Z", 0},
		{"e", "Eve", "2025-12-01T00:00:00Z", -1},
		{"f", "Frank", "2026-01-15T00:00:00Z", 9_000_002},
	} {
		if _, err := s.DB.Exec("INSERT INTO users(id,name,email,created_at) VALUES(?,?,?,?)", customer.id, customer.name, customer.id+"@example.com", customer.createdAt); err != nil {
			t.Fatal(err)
		}
		if customer.balance >= 0 {
			if _, err := s.DB.Exec("INSERT INTO wallets(user_id,balance_micros) VALUES(?,?)", customer.id, customer.balance); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, tc := range []struct {
		name, query string
		want        []string
		total       int64
	}{
		{"default", "", []string{"d", "c", "b", "f", "a", "e"}, 6},
		{"newest", "sort=-created_at", []string{"d", "c", "b", "f", "a", "e"}, 6},
		{"oldest", "sort=created_at", []string{"e", "a", "f", "c", "b", "d"}, 6},
		{"highest_balance", "sort=-balance", []string{"c", "b", "f", "a", "e", "d"}, 6},
		{"lowest_balance", "sort=balance", []string{"e", "d", "a", "f", "c", "b"}, 6},
		{"second_page", "sort=-balance&page[size]=2&page[number]=2", []string{"f", "a"}, 6},
		{"tie_across_pages", "sort=created_at&page[size]=1&page[number]=5", []string{"b"}, 6},
		{"search_name", "sort=balance&q=mAtCh", []string{"a", "c", "b"}, 3},
		{"search_page", "sort=-balance&q=match&page[size]=1&page[number]=2", []string{"b"}, 3},
		{"search_email", "sort=created_at&q=f%40example.com", []string{"f"}, 1},
		{"no_matches", "sort=balance&q=missing", []string{}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := adminTestDecode[portal.Customer](t, adminTestGet(t, s, "/admin/customers?"+tc.query, "admin"))
			ids := []string{}
			for _, item := range doc.Data {
				ids = append(ids, item.ID)
				if item.Attributes.CreatedAt.IsZero() {
					t.Fatal("missing registration date")
				}
				if (item.ID == "d" || item.ID == "e") && item.Attributes.Balance != 0 {
					t.Fatalf("balance for %s = %s, want zero", item.ID, item.Attributes.Balance)
				}
			}
			if !slices.Equal(ids, tc.want) || doc.Meta.Total != tc.total {
				t.Fatalf("ids = %v, total = %d; want %v, %d", ids, doc.Meta.Total, tc.want, tc.total)
			}
			params, _ := url.ParseQuery(tc.query)
			for _, link := range doc.Links {
				u, err := url.Parse(link)
				if err != nil || u.Query().Get("sort") != params.Get("sort") || u.Query().Get("q") != params.Get("q") {
					t.Fatalf("pagination link lost sort or search: %s", link)
				}
			}
		})
	}
	for _, sort := range []string{"email", "balance DESC", "balance,-created_at", "-balance;DROP TABLE users"} {
		t.Run("invalid_"+sort, func(t *testing.T) {
			w := adminTestGet(t, s, "/admin/customers?sort="+url.QueryEscape(sort), "admin")
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", w.Code)
			}
		})
	}
}

func TestAdminOrdersCustomerIdentity(t *testing.T) {
	s := adminTestServer(t)
	for _, customer := range []struct{ id, name string }{{"alice", "Alice"}, {"bob", ""}} {
		if _, err := s.DB.Exec("INSERT INTO users(id,name,email) VALUES(?,?,?)", customer.id, customer.name, customer.id+"@example.com"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.DB.Exec("INSERT INTO wallets(user_id) VALUES(?)", customer.id); err != nil {
			t.Fatal(err)
		}
	}
	for i, userID := range []string{"alice", "bob", "alice"} {
		if _, err := s.DB.Exec("INSERT INTO payment_orders(id,user_id,provider,amount_micros,idempotency_key,created_at) VALUES(?,?,'dummy',10000000,?,?)", string(rune('a'+i)), userID, string(rune('a'+i)), "2026-01-01T00:00:00Z"); err != nil {
			t.Fatal(err)
		}
	}
	doc := adminTestDecode[wallet.Order](t, adminTestGet(t, s, "/admin/orders", "admin"))
	if len(doc.Data) != 3 || doc.Meta.Total != 3 {
		t.Fatalf("orders = %d, total = %d; want 3", len(doc.Data), doc.Meta.Total)
	}
	for _, item := range doc.Data {
		order := item.Attributes
		wantName := "Alice"
		if order.UserID == "bob" {
			wantName = ""
		}
		if order.UserName != wantName || order.UserEmail != order.UserID+"@example.com" {
			t.Fatalf("incorrect order customer: %+v", order)
		}
	}
	page := adminTestDecode[wallet.Order](t, adminTestGet(t, s, "/admin/orders?page[size]=1&page[number]=2", "admin"))
	if len(page.Data) != 1 || page.Data[0].ID != "b" || page.Data[0].Attributes.UserEmail != "bob@example.com" || page.Meta.Total != 3 {
		t.Fatalf("incorrect paginated order: %+v", page)
	}
	empty := adminTestDecode[wallet.Order](t, adminTestGet(t, s, "/admin/orders?page[size]=1&page[number]=4", "admin"))
	if len(empty.Data) != 0 || empty.Meta.Total != 3 {
		t.Fatalf("incorrect empty page: %+v", empty)
	}
	customerOrders := adminTestDecode[map[string]json.RawMessage](t, adminTestGet(t, s, "/orders", "alice"))
	if len(customerOrders.Data) != 2 || customerOrders.Meta.Total != 2 {
		t.Fatalf("customer orders = %d, total = %d; want 2", len(customerOrders.Data), customerOrders.Meta.Total)
	}
	for _, item := range customerOrders.Data {
		var userID string
		if err := json.Unmarshal(item.Attributes["user_id"], &userID); err != nil {
			t.Fatal(err)
		}
		if userID != "alice" || item.Attributes["user_name"] != nil || item.Attributes["user_email"] != nil {
			t.Fatalf("customer order changed or leaked identity: %s", item.Attributes)
		}
	}
	for _, userID := range []string{"", "alice"} {
		for _, path := range []string{"/admin/orders", "/admin/customers"} {
			want := http.StatusForbidden
			if userID == "" {
				want = http.StatusUnauthorized
			}
			if w := adminTestGet(t, s, path, userID); w.Code != want {
				t.Fatalf("%s as %q: status = %d, want %d", path, userID, w.Code, want)
			}
		}
	}
}
