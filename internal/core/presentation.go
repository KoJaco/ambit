package core

import (
	"fmt"
	"strings"
)

var allowedIcons = map[string]bool{
	"box": true, "hexagon": true, "layers": true, "component": true, "package": true,
	"server": true, "cpu": true, "cloud": true, "globe": true, "workflow": true,
	"database": true, "hard-drive": true, "archive": true, "table": true,
	"file-text": true, "scroll-text": true, "book-open": true, "sticky-note": true,
	"users": true, "shield": true, "lock": true,
	"git-branch": true, "share-2": true, "network": true,
}

var allowedColors = map[string]bool{
	"neutral": true, "slate": true, "blue": true, "teal": true, "green": true,
	"amber": true, "violet": true, "rose": true,
}

var allowedRenderers = map[string]bool{
	"card": true, "schema": true, "placeholder": true, "group": true,
}

const (
	maxSchemaRows  = 32
	maxSchemaField = 64
)

// ValidIcon reports whether id is in the closed icon registry.
func ValidIcon(id string) bool {
	return allowedIcons[id]
}

// ValidColor reports whether token is in the closed colour palette.
func ValidColor(token string) bool {
	return allowedColors[token]
}

func validateRegistryValue(registry map[string]bool, field, new, current string) error {
	if new == "" {
		return nil
	}
	if registry[new] {
		return nil
	}
	if new == current {
		return nil
	}
	return fmt.Errorf("%w: unknown %s %q", ErrInvalidPresentation, field, new)
}

func validateIconWrite(new, current string) error {
	return validateRegistryValue(allowedIcons, "icon", new, current)
}

func validateColorWrite(new, current string) error {
	return validateRegistryValue(allowedColors, "color", new, current)
}

// applyIconPatch sets n.Icon when patch is non-nil. Empty string clears the field.
func applyIconPatch(n *Node, patch *string) error {
	if patch == nil {
		return nil
	}
	next := strings.TrimSpace(*patch)
	if err := validateIconWrite(next, n.Icon); err != nil {
		return err
	}
	n.Icon = next
	return nil
}

func applyColorPatch(n *Node, patch *string) error {
	if patch == nil {
		return nil
	}
	next := strings.TrimSpace(*patch)
	if err := validateColorWrite(next, n.Color); err != nil {
		return err
	}
	n.Color = next
	return nil
}

func validateRendererWrite(new, current string) error {
	return validateRegistryValue(allowedRenderers, "renderer", new, current)
}

func validateSchema(cols []SchemaColumn) error {
	if len(cols) > maxSchemaRows {
		return fmt.Errorf("%w: schema has %d rows (max %d)", ErrInvalidPresentation, len(cols), maxSchemaRows)
	}
	for i, col := range cols {
		title := strings.TrimSpace(col.Title)
		typ := strings.TrimSpace(col.Type)
		if title == "" || typ == "" {
			return fmt.Errorf("%w: schema row %d needs title and type", ErrInvalidPresentation, i+1)
		}
		if len(title) > maxSchemaField || len(typ) > maxSchemaField {
			return fmt.Errorf("%w: schema row %d exceeds %d characters", ErrInvalidPresentation, i+1, maxSchemaField)
		}
	}
	return nil
}

func normalizeSchema(cols []SchemaColumn) []SchemaColumn {
	if len(cols) == 0 {
		return nil
	}
	out := make([]SchemaColumn, len(cols))
	for i, col := range cols {
		out[i] = SchemaColumn{Title: strings.TrimSpace(col.Title), Type: strings.TrimSpace(col.Type)}
	}
	return out
}

func applyRendererPatch(n *Node, patch *string) error {
	if patch == nil {
		return nil
	}
	next := strings.TrimSpace(*patch)
	if err := validateRendererWrite(next, n.Renderer); err != nil {
		return err
	}
	n.Renderer = next
	return nil
}

func applySchemaPatch(n *Node, patch *[]SchemaColumn) error {
	if patch == nil {
		return nil
	}
	next := normalizeSchema(*patch)
	if err := validateSchema(next); err != nil {
		return err
	}
	n.Schema = next
	return nil
}

func validateCreatePresentation(icon, color, renderer string, schema []SchemaColumn) error {
	if err := validateIconWrite(icon, ""); err != nil {
		return err
	}
	if err := validateColorWrite(color, ""); err != nil {
		return err
	}
	if err := validateRendererWrite(renderer, ""); err != nil {
		return err
	}
	return validateSchema(normalizeSchema(schema))
}
