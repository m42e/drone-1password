// Copyright 2019 the Drone Authors. All rights reserved.
// Use of this source code is governed by the Blue Oak Model License
// that can be found in the LICENSE file.

package plugin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/drone/drone-go/drone"
	"github.com/drone/drone-go/plugin/secret"
	"github.com/sirupsen/logrus"
)

type Config struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	Logger     logrus.FieldLogger
}

type plugin struct {
	client *connectClient
	logger logrus.FieldLogger
}

func New(cfg Config) (secret.Plugin, error) {
	client, err := newConnectClient(cfg.BaseURL, cfg.Token, cfg.HTTPClient)
	if err != nil {
		return nil, err
	}
	logger := cfg.Logger
	if logger == nil {
		logger = logrus.New()
	}
	return &plugin{
		client: client,
		logger: logger,
	}, nil
}

func (p *plugin) Find(ctx context.Context, req *secret.Request) (*drone.Secret, error) {
	if req == nil {
		p.logger.Error("secret request failed: nil request")
		return nil, errors.New("nil request")
	}
	entry := p.logger.WithFields(logrus.Fields{
		"secret": req.Name,
		"path":   req.Path,
	})
	entry.Info("secret request received")
	if req.Name == "" {
		err := errors.New("secret name must not be empty")
		entry.WithError(err).Error("secret request failed")
		return nil, err
	}
	vaultName, itemTitle, fieldSelector, err := parseSecretPath(req.Path)
	if err != nil {
		entry.WithError(err).Error("secret request failed")
		return nil, err
	}

	vault, err := p.client.findVaultByName(ctx, vaultName)
	if err != nil {
		err = fmt.Errorf("lookup vault %q: %w", vaultName, err)
		entry.WithError(err).Error("secret request failed")
		return nil, err
	}
	itemSummary, err := p.client.findItemByTitle(ctx, vault.ID, itemTitle)
	if err != nil {
		err = fmt.Errorf("lookup item %q: %w", itemTitle, err)
		entry.WithError(err).Error("secret request failed")
		return nil, err
	}
	item, err := p.client.getItem(ctx, vault.ID, itemSummary.ID)
	if err != nil {
		err = fmt.Errorf("load item %q: %w", itemTitle, err)
		entry.WithError(err).Error("secret request failed")
		return nil, err
	}

	value, err := selectFieldValue(item, fieldSelector)
	if err != nil {
		entry.WithError(err).Error("secret request failed")
		return nil, err
	}

	entry.WithFields(logrus.Fields{
		"vault": vault.Name,
		"item":  item.Title,
		"field": fieldSelector,
	}).Info("secret request succeeded")

	return &drone.Secret{
		Name:        req.Name,
		Data:        value,
		PullRequest: false,
	}, nil
}

func parseSecretPath(path string) (vault, item, field string, err error) {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("secret path must be formatted as vault/item[/field]")
	}
	vault = strings.TrimSpace(parts[0])
	item = strings.TrimSpace(parts[1])
	if vault == "" || item == "" {
		return "", "", "", fmt.Errorf("vault and item names cannot be empty")
	}
	if len(parts) == 3 {
		field = strings.TrimSpace(parts[2])
	}
	return vault, item, field, nil
}

func selectFieldValue(item *fullItem, selector string) (string, error) {
	if selector == "" {
		return defaultPassword(item)
	}
	if strings.EqualFold(selector, "notes") || strings.EqualFold(selector, "notesPlain") {
		if item.NotesPlain == "" {
			return "", fmt.Errorf("item %q does not contain notes", item.Title)
		}
		return item.NotesPlain, nil
	}
	sectionName, fieldLabel := splitQualifiedLabel(selector)
	if sectionName != "" {
		return findFieldInSection(item, sectionName, fieldLabel)
	}
	return findFieldByLabel(item, fieldLabel)
}

func defaultPassword(item *fullItem) (string, error) {
	var matches []string
	for i := range item.Fields {
		field := item.Fields[i]
		if !strings.EqualFold(field.Purpose, "PASSWORD") {
			continue
		}
		if field.Value == "" {
			continue
		}
		matches = append(matches, field.Value)
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return findFieldByLabel(item, "password")
	default:
		return "", fmt.Errorf("item %q defines multiple password fields; specify the desired field label", item.Title)
	}
}

func findFieldByLabel(item *fullItem, label string) (string, error) {
	fieldLabel := strings.TrimSpace(label)
	var matches []itemField
	for i := range item.Fields {
		field := item.Fields[i]
		if !strings.EqualFold(field.Label, fieldLabel) {
			continue
		}
		if field.Value == "" {
			continue
		}
		matches = append(matches, field)
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("field %q not found in item %q", fieldLabel, item.Title)
	case 1:
		return matches[0].Value, nil
	default:
		return "", fmt.Errorf("field label %q is ambiguous in item %q; use a section-qualified label", fieldLabel, item.Title)
	}
}

func findFieldInSection(item *fullItem, sectionLabel, fieldLabel string) (string, error) {
	sectionIDs := make(map[string]struct{})
	for i := range item.Sections {
		section := item.Sections[i]
		if strings.EqualFold(section.Label, sectionLabel) {
			sectionIDs[section.ID] = struct{}{}
		}
	}
	if len(sectionIDs) == 0 {
		return "", fmt.Errorf("section %q not found in item %q", sectionLabel, item.Title)
	}
	var matches []itemField
	for i := range item.Fields {
		field := item.Fields[i]
		if field.Section == nil {
			continue
		}
		if _, ok := sectionIDs[field.Section.ID]; !ok {
			continue
		}
		if !strings.EqualFold(field.Label, fieldLabel) {
			continue
		}
		if field.Value == "" {
			continue
		}
		matches = append(matches, field)
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("field %q not found in section %q", fieldLabel, sectionLabel)
	case 1:
		return matches[0].Value, nil
	default:
		return "", fmt.Errorf("field %q is duplicated in section %q", fieldLabel, sectionLabel)
	}
}

func splitQualifiedLabel(label string) (section, field string) {
	parts := strings.SplitN(label, "/", 2)
	if len(parts) == 1 {
		return "", strings.TrimSpace(parts[0])
	}
	section = strings.TrimSpace(parts[0])
	field = strings.TrimSpace(parts[1])
	return
}
