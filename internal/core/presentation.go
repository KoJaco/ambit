package core

import (
	"fmt"
	"strings"
)

// IconInfo is one entry of the closed icon registry. Every ID must exist in lucide-react
// and in frontend/app/node-presentation.ts ICON_COMPONENTS.
type IconInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

var iconRegistry = []IconInfo{
	{"box", "Generic component or unit"},
	{"hexagon", "Core module or domain"},
	{"layers", "Layered stack or tier"},
	{"component", "UI or reusable component"},
	{"package", "Library, package or bundle"},
	{"server", "Server or backend service"},
	{"cpu", "Compute, worker or processor"},
	{"cloud", "Cloud platform or hosted zone"},
	{"globe", "Public web, internet or CDN"},
	{"workflow", "Pipeline or workflow"},
	{"database", "Database"},
	{"hard-drive", "Storage volume or disk"},
	{"archive", "Archive, backup or cold storage"},
	{"table", "Table or tabular data"},
	{"file-text", "Document or file"},
	{"scroll-text", "Log, ledger or spec"},
	{"book-open", "Docs or reference"},
	{"sticky-note", "Note or annotation"},
	{"users", "Users, team or actors"},
	{"shield", "Security or protection"},
	{"lock", "Secrets, access control or encryption"},
	{"git-branch", "Branching or version control"},
	{"share-2", "Fan-out, sharing or pub/sub"},
	{"network", "Network or topology"},
	{"smartphone", "Mobile app or device"},
	{"monitor", "Web app, browser or desktop client"},
	{"key-round", "Auth, keys or identity"},
	{"mail", "Email or messaging"},
	{"bell", "Notifications or alerts"},
	{"message-square", "Chat or message channel"},
	{"credit-card", "Payments or billing"},
	{"folder", "Folder or grouping"},
	{"code", "Source code or SDK"},
	{"terminal", "CLI or shell"},
	{"zap", "Serverless function or event trigger"},
	{"clock", "Scheduler, cron or timer"},
	{"settings", "Configuration or admin"},
	{"bot", "Agent, bot or AI model"},
	{"search", "Search or indexing"},
	{"cable", "Integration, connector or adapter"},
}

var allowedIcons = func() map[string]bool {
	m := make(map[string]bool, len(iconRegistry))
	for _, i := range iconRegistry {
		m[i.ID] = true
	}
	return m
}()

// Icons returns the icon registry in display order.
func Icons() []IconInfo {
	return append([]IconInfo(nil), iconRegistry...)
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
