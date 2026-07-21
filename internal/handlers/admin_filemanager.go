package handlers

import (
	"cms-go/internal/db"
	"cms-go/internal/models"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

// Allowed root directories for the file manager.
var fmRoots = []string{"assets", "internal/views"}

// FileEntry is a file or directory shown in the file manager tree.
type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size,omitempty"`
	SizeStr string `json:"sizeStr,omitempty"`
	ModTime string `json:"modTime,omitempty"`
	Ext     string `json:"ext,omitempty"`
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	return fmt.Sprintf("%d KB", bytes/1024)
}

func AdminFileManager(c echo.Context) error {
	sub := strings.TrimSpace(c.QueryParam("path"))
	files, parentPath, err := listFiles(sub)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	// Walk the allowed roots to build a directory tree for the sidebar.
	tree := buildTree()

	data := map[string]interface{}{
		"Files":      files,
		"ParentPath": parentPath,
		"CurrentDir": sub,
		"Tree":       tree,
	}
	return renderWithLayout(c,
		"internal/views/admin/admin-layout.html",
		"internal/views/admin/filemanager.html",
		data)
}

func AdminFileEdit(c echo.Context) error {
	path := c.Param("*")
	if path == "" || strings.HasPrefix(path, "/") {
		path = strings.TrimPrefix(c.QueryParam("path"), "/")
	}
	if path == "" {
		return AdminFileManager(c)
	}

	safe, rel, err := resolvePath(path)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	info, err := os.Stat(safe)
	if err != nil {
		return c.String(http.StatusNotFound, "File not found")
	}
	if info.IsDir() {
		// Redirect to directory listing
		return c.Redirect(http.StatusSeeOther, "/admin/file-manager?path="+rel)
	}

	content, err := os.ReadFile(safe)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Cannot read file")
	}

	// Build tree for sidebar
	tree := buildTree()
	// Also list current directory siblings for context
	parentDir := filepath.Dir(rel)
	parentFiles, _, _ := listFiles(parentDir)

	ext := filepath.Ext(safe)

	data := map[string]interface{}{
		"FilePath":    rel,
		"FileName":    info.Name(),
		"FileContent": string(content),
		"FileExt":     ext,
		"Tree":        tree,
		"Files":       parentFiles,
		"CurrentDir":  parentDir,
		"ParentPath":  parentDir,
	}
	return renderWithLayout(c,
		"internal/views/admin/admin-layout.html",
		"internal/views/admin/filemanager.html",
		data)
}

func AdminFileSave(c echo.Context) error {
	path := strings.TrimSpace(c.FormValue("path"))
	content := c.FormValue("content")

	if path == "" {
		return c.String(http.StatusBadRequest, "Missing path")
	}

	safe, _, err := resolvePath(path)
	if err != nil {
		return c.String(http.StatusForbidden, err.Error())
	}

	info, err := os.Stat(safe)
	if err == nil && info.IsDir() {
		return c.String(http.StatusBadRequest, "Cannot write to a directory")
	}

	if err := os.WriteFile(safe, []byte(content), 0644); err != nil {
		return c.String(http.StatusInternalServerError, "Cannot write file: "+err.Error())
	}

	return c.Redirect(http.StatusSeeOther, "/admin/file-manager/edit/"+path)
}

// Delete file route
func AdminFileDelete(c echo.Context) error {
	path := strings.TrimSpace(c.FormValue("path"))
	if path == "" {
		return c.String(http.StatusBadRequest, "Missing path")
	}

	safe, _, err := resolvePath(path)
	if err != nil {
		return c.String(http.StatusForbidden, err.Error())
	}

	if err := os.Remove(safe); err != nil {
		return c.String(http.StatusInternalServerError, "Cannot delete: "+err.Error())
	}

	dir := filepath.Dir(path)
	return c.Redirect(http.StatusSeeOther, "/admin/file-manager?path="+dir)
}

// resolvePath validates that the requested path is within one of the allowed
// root directories and returns the absolute safe path + the relative path.
func resolvePath(requested string) (safePath, relativePath string, err error) {
	// Sanitize: strip leading slashes, prevent traversal
	requested = filepath.Clean(strings.TrimPrefix(requested, "/"))
	if requested == "." || requested == "" {
		requested = ""
	}

	// Check against allowed roots
	for _, root := range fmRoots {
		base := filepath.Clean(root)
		candidate := filepath.Join(base, requested)

		// Prevent escaping the root
		absCandidate, _ := filepath.Abs(candidate)
		absRoot, _ := filepath.Abs(base)

		// Ensure candidate is within root
		if !strings.HasPrefix(absCandidate, absRoot) {
			continue
		}

		// Check it doesn't contain .. tricks after resolution
		if strings.Contains(requested, "..") {
			continue
		}

		return candidate, filepath.Join(root, requested), nil
	}

	return "", "", echo.NewHTTPError(http.StatusForbidden, "Path not allowed")
}

// listFiles lists files in a subdirectory relative to the allowed roots.
func listFiles(sub string) ([]FileEntry, string, error) {
	if sub == "" {
		// Show root directories
		var entries []FileEntry
		for _, r := range fmRoots {
			info, err := os.Stat(r)
			if err != nil {
				continue
			}
			entries = append(entries, FileEntry{
				Name:    r,
				Path:    r,
				IsDir:   info.IsDir(),
				ModTime: info.ModTime().Format("Jan 02 15:04"),
			})
		}
		return entries, "", nil
	}

	// Try each root prefix
	for _, root := range fmRoots {
		base := filepath.Clean(root)
		dir := filepath.Join(base, sub)

		absDir, _ := filepath.Abs(dir)
		absRoot, _ := filepath.Abs(base)
		if !strings.HasPrefix(absDir, absRoot) {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		var files []FileEntry
		for _, e := range entries {
			info, _ := e.Info()
			size := int64(0)
			modTime := ""
			if info != nil {
				size = info.Size()
				modTime = info.ModTime().Format("Jan 02 15:04")
			}
			files = append(files, FileEntry{
				Name:    e.Name(),
				Path:    filepath.Join(sub, e.Name()),
				IsDir:   e.IsDir(),
				Size:    size,
				SizeStr: formatSize(size),
				ModTime: modTime,
				Ext:     filepath.Ext(e.Name()),
			})
		}

		// parent path for "go up"
		parentPath := filepath.Dir(sub)
		if parentPath == "." {
			parentPath = ""
		}
		// Only allow parent if it's still within our roots
		allowed := false
		for _, r := range fmRoots {
			if strings.HasPrefix(filepath.Join(r, parentPath)+"/", filepath.Join(r)+"/") {
				allowed = true
				break
			}
		}
		if !allowed {
			parentPath = ""
		}

		return files, parentPath, nil
	}

	return nil, "", echo.NewHTTPError(http.StatusNotFound, "Directory not found")
}

// TreeNode represents a node in the directory tree sidebar.
type TreeNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	IsDir    bool       `json:"isDir"`
	Children []TreeNode `json:"children,omitempty"`
	Open     bool       `json:"open,omitempty"`
}

// buildTree walks the allowed roots and builds a nested tree for the sidebar.
func buildTree() []TreeNode {
	var roots []TreeNode
	for _, r := range fmRoots {
		roots = append(roots, walkDir(r, r, 0))
	}
	return roots
}

func walkDir(rootPath, displayPath string, depth int) TreeNode {
	maxDepth := 3
	node := TreeNode{
		Name:  filepath.Base(rootPath),
		Path:  rootPath,
		IsDir: true,
	}

	if depth >= maxDepth {
		return node
	}

	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return node
	}

	for _, e := range entries {
		childPath := filepath.Join(rootPath, e.Name())
		childDisplay := filepath.Join(displayPath, e.Name())
		if e.IsDir() {
			child := walkDir(childPath, childDisplay, depth+1)
			node.Children = append(node.Children, child)
		} else {
			node.Children = append(node.Children, TreeNode{
				Name:  e.Name(),
				Path:  childDisplay,
				IsDir: false,
			})
		}
	}

	return node
}

// SeedFileManagerMenu creates the file-manager menu entry and grants it to
// the admin role (role_id=1). Called from router.go after DB migration.
func SeedFileManagerMenu() {
	var count int64
	db.DB.Model(&models.Menu{}).Where("path = '/admin/file-manager'").Count(&count)
	if count > 0 {
		return
	}
	menu := models.Menu{
		Menu:      "File Manager",
		Path:      "/admin/file-manager",
		Icon:      "📁",
		MenuType:  "module",
		ListOrder: 90,
		Status:    1,
	}
	db.DB.Create(&menu)

	// Grant full permissions to admin role (role_id=1)
	db.DB.Where("role_id = 1 AND menu_id = ?", menu.ID).Delete(&models.Permission{})
	db.DB.Create(&models.Permission{
		RoleID:    1,
		MenuID:    menu.ID,
		CanCreate: true,
		CanRead:   true,
		CanUpdate: true,
		CanDelete: true,
	})
}
