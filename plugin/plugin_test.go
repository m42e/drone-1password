// Copyright 2019 the Drone Authors. All rights reserved.
// Use of this source code is governed by the Blue Oak Model License
// that can be found in the LICENSE file.

package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/drone/drone-go/drone"
	"github.com/drone/drone-go/plugin/secret"
	"github.com/sirupsen/logrus"
)

func TestParseSecretPath(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantVault string
		wantItem  string
		wantField string
		wantErr   bool
	}{
		{"basic", "Vault/Item", "Vault", "Item", "", false},
		{"with field", "Vault/Item/Field", "Vault", "Item", "Field", false},
		{"trim spaces", " Vault / Item / Field ", "Vault", "Item", "Field", false},
		{"missing item", "Vault", "", "", "", true},
		{"empty segments", "Vault/ /Field", "", "", "", true},
	}

	for _, tc := range tests {
		vault, item, field, err := parseSecretPath(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("%s: expected error", tc.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if vault != tc.wantVault || item != tc.wantItem || field != tc.wantField {
			t.Fatalf("%s: got %q/%q/%q", tc.name, vault, item, field)
		}
	}
}

func TestSelectFieldValue(t *testing.T) {
	item := &fullItem{
		Title: "Sample",
		Fields: []itemField{
			{Label: "Username", Value: "octocat"},
			{Label: "Password", Value: "hunter2", Purpose: "PASSWORD"},
			{Label: "Token", Value: "abc123", Section: &itemFieldSection{ID: "sec"}},
		},
		Sections: []itemSection{
			{ID: "sec", Label: "Service Keys"},
		},
		NotesPlain: "secret note",
	}

	value, err := selectFieldValue(item, "")
	if err != nil || value != "hunter2" {
		t.Fatalf("default password lookup failed: %v value=%q", err, value)
	}

	value, err = selectFieldValue(item, "Password")
	if err != nil || value != "hunter2" {
		t.Fatalf("label lookup failed: %v value=%q", err, value)
	}

	value, err = selectFieldValue(item, "Service Keys / Token")
	if err != nil || value != "abc123" {
		t.Fatalf("section-qualified lookup failed: %v value=%q", err, value)
	}

	value, err = selectFieldValue(item, "notes")
	if err != nil || value != "secret note" {
		t.Fatalf("notes lookup failed: %v value=%q", err, value)
	}
}

func TestPluginFind(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/v1/vaults":
			checkQuery(t, r, "name eq \"Production Vault\"")
			json.NewEncoder(w).Encode([]vaultSummary{{ID: "vault-id", Name: "Production Vault"}})
		case "/v1/vaults/vault-id/items":
			checkQuery(t, r, "title eq \"Database Credentials\"")
			json.NewEncoder(w).Encode([]itemSummary{{ID: "item-id", Title: "Database Credentials"}})
		case "/v1/vaults/vault-id/items/item-id":
			json.NewEncoder(w).Encode(fullItem{
				Title:  "Database Credentials",
				Fields: []itemField{{Label: "Password", Value: "hunter2", Purpose: "PASSWORD"}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	plug, err := New(Config{
		BaseURL:    server.URL,
		Token:      "token",
		HTTPClient: server.Client(),
		Logger:     logger,
	})
	if err != nil {
		t.Fatalf("failed to create plugin: %v", err)
	}

	secretValue, err := plug.Find(context.Background(), &secret.Request{
		Name: "db_password",
		Path: "Production Vault/Database Credentials",
		Repo: drone.Repo{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secretValue == nil || secretValue.Data != "hunter2" {
		t.Fatalf("unexpected secret: %#v", secretValue)
	}
}

func checkQuery(t *testing.T, r *http.Request, want string) {
	t.Helper()
	if got := r.URL.Query().Get("filter"); got != want {
		t.Fatalf("unexpected filter: got %q want %q", got, want)
	}
}
