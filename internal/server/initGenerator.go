package server

import (
	"cms-go/internal/db"
	"cms-go/internal/models"
	"fmt"
	"os"
	"path/filepath"
)

// GenerateTemplatesFromDB fetch all layouts and components then save to views/generated
func GenerateTemplatesFromDB() error {
	// Ensure base generated folders exist
	baseDir := "internal/views/generated"
	compDir := filepath.Join(baseDir, "components")
	layoutDir := filepath.Join(baseDir, "layouts")

	if err := os.MkdirAll(compDir, 0755); err != nil {
		return fmt.Errorf("failed to create components dir: %w", err)
	}
	if err := os.MkdirAll(layoutDir, 0755); err != nil {
		return fmt.Errorf("failed to create layouts dir: %w", err)
	}

	// --- Components ---
	var comps []models.Component
	if err := db.DB.Find(&comps).Error; err != nil {
		return fmt.Errorf("fetch components: %w", err)
	}
	for _, comp := range comps {
		if comp.Template == "" {
			continue
		}
		filePath := filepath.Join(compDir, comp.Name+".html")
		content := fmt.Sprintf(`{{ define "%s" }}%s{{ end }}`, comp.Name, comp.Template)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			fmt.Printf("failed to write component %s: %v\n", comp.Name, err)
		}
	}

	// --- Layouts ---
	var layouts []models.Layout
	if err := db.DB.Find(&layouts).Error; err != nil {
		return fmt.Errorf("fetch layouts: %w", err)
	}
	for _, layout := range layouts {
		if layout.Template == "" {
			continue
		}
		filePath := filepath.Join(layoutDir, fmt.Sprintf("layout-%d.html", layout.ID))
		if err := os.WriteFile(filePath, []byte(layout.Template), 0644); err != nil {
			fmt.Printf("failed to write layout %d: %v\n", layout.ID, err)
		}
	}

	fmt.Println("✅ Generated templates from DB into", baseDir)
	return nil
}
