package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// MenuNode is one entry in the tree rendered for a menu group: a menu plus
// its children, in list_order.
type MenuNode struct {
	Menu     models.Menu
	Children []MenuNode
}

func buildMenuTree(menus []models.Menu) []MenuNode {
	childrenOf := map[uint][]models.Menu{}
	for _, m := range menus {
		childrenOf[m.ParentID] = append(childrenOf[m.ParentID], m)
	}
	var build func(parentID uint) []MenuNode
	build = func(parentID uint) []MenuNode {
		kids := childrenOf[parentID]
		nodes := make([]MenuNode, 0, len(kids))
		for _, m := range kids {
			nodes = append(nodes, MenuNode{Menu: m, Children: build(m.ID)})
		}
		return nodes
	}
	return build(0)
}

// resolveGroup loads the requested group (by id, via ?group=) or falls back
// to the system "Backend Menu" group.
func resolveGroup(c echo.Context) (models.MenuGroup, error) {
	var group models.MenuGroup
	if idStr := c.QueryParam("group"); idStr != "" {
		if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
			if err := db.DB.First(&group, id).Error; err == nil {
				return group, nil
			}
		}
	}
	if err := db.DB.Where("is_system = ?", true).First(&group).Error; err != nil {
		return group, err
	}
	return group, nil
}

// GET /admin/menus?group=N
func AdminMenus(c echo.Context) error {
	group, err := resolveGroup(c)
	if err != nil {
		return c.String(http.StatusInternalServerError, "No menu group available")
	}

	var groups []models.MenuGroup
	db.DB.Order("is_system desc, name asc").Find(&groups)

	var menus []models.Menu
	db.DB.Where("menu_group_id = ?", group.ID).Order("list_order asc, id asc").Find(&menus)

	data := map[string]interface{}{
		"Groups":        groups,
		"SelectedGroup": group,
		"MenuTree":      buildMenuTree(menus),
		"HasMenus":      len(menus) > 0,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/menus.html", data)
}

// POST /admin/menus/groups — creates a new menu location (e.g. "Landing Menu").
func AdminCreateMenuGroup(c echo.Context) error {
	name := strings.TrimSpace(c.FormValue("name"))
	if name == "" {
		return c.String(http.StatusBadRequest, "Menu group name is required")
	}

	slug := slugify(name)
	if slug == "" {
		slug = "menu"
	}
	base := slug
	for i := 2; ; i++ {
		var count int64
		db.DB.Model(&models.MenuGroup{}).Where("slug = ?", slug).Count(&count)
		if count == 0 {
			break
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}

	group := models.MenuGroup{Name: name, Slug: slug}
	if err := db.DB.Create(&group).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to create menu group")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/menus?group="+strconv.FormatUint(uint64(group.ID), 10))
}

// POST /admin/menus/groups/:id/delete
func AdminDeleteMenuGroup(c echo.Context) error {
	var group models.MenuGroup
	if err := db.DB.First(&group, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Menu group not found")
	}
	if group.IsSystem {
		return c.String(http.StatusBadRequest, "Cannot delete the system Backend Menu group")
	}

	var itemCount int64
	db.DB.Model(&models.Menu{}).Where("menu_group_id = ?", group.ID).Count(&itemCount)
	if itemCount > 0 {
		return c.String(http.StatusBadRequest, "Cannot delete a menu group that still has items")
	}

	db.DB.Delete(&group)
	return c.Redirect(http.StatusSeeOther, "/admin/menus")
}

func bindMenuFromForm(c echo.Context, menu *models.Menu) {
	menu.Menu = c.FormValue("menu")
	menu.Path = c.FormValue("path")
	menu.Icon = c.FormValue("icon")
	menu.MenuDescription = c.FormValue("menu_description")
	menu.MenuType = c.FormValue("menu_type")
	if c.FormValue("status") == "on" {
		menu.Status = 1
	} else {
		menu.Status = 0
	}
}

// POST /admin/menus/new — always creates at the root of the submitted
// group; use the tree's drag-and-drop to nest or reorder afterward.
func AdminCreateMenu(c echo.Context) error {
	groupID, err := strconv.ParseUint(c.FormValue("group_id"), 10, 64)
	if err != nil || groupID == 0 {
		return c.String(http.StatusBadRequest, "Missing menu group")
	}
	var group models.MenuGroup
	if err := db.DB.First(&group, groupID).Error; err != nil {
		return c.String(http.StatusNotFound, "Menu group not found")
	}

	var menu models.Menu
	bindMenuFromForm(c, &menu)
	if menu.Menu == "" || menu.Path == "" {
		return c.String(http.StatusBadRequest, "Menu name and path are required")
	}
	menu.MenuGroupID = group.ID
	menu.ParentID = 0

	var maxOrder uint32
	db.DB.Model(&models.Menu{}).Where("menu_group_id = ?", group.ID).
		Select("COALESCE(MAX(list_order), 0)").Scan(&maxOrder)
	menu.ListOrder = maxOrder + 1

	if err := db.DB.Create(&menu).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to create menu")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/menus?group="+strconv.FormatUint(uint64(group.ID), 10))
}

// POST /admin/menus/:id/edit — content-only update; structure (parent/order)
// is changed exclusively via drag-and-drop through AdminReorderMenus.
func AdminUpdateMenu(c echo.Context) error {
	var menu models.Menu
	if err := db.DB.First(&menu, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Menu not found")
	}
	bindMenuFromForm(c, &menu)
	if err := db.DB.Save(&menu).Error; err != nil {
		return c.String(http.StatusBadRequest, "Failed to update menu")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/menus?group="+strconv.FormatUint(uint64(menu.MenuGroupID), 10))
}

// POST /admin/menus/:id/delete
func AdminDeleteMenu(c echo.Context) error {
	var menu models.Menu
	if err := db.DB.First(&menu, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Menu not found")
	}

	var childCount int64
	db.DB.Model(&models.Menu{}).Where("parent_id = ?", menu.ID).Count(&childCount)
	if childCount > 0 {
		return c.String(http.StatusBadRequest, "Cannot delete a menu that still has children")
	}

	groupID := menu.MenuGroupID
	db.DB.Delete(&menu)
	db.DB.Delete(&models.Permission{}, "menu_id = ?", menu.ID)
	return c.Redirect(http.StatusSeeOther, "/admin/menus?group="+strconv.FormatUint(uint64(groupID), 10))
}

type reorderItem struct {
	ID       uint   `json:"id"`
	ParentID uint   `json:"parent_id"`
	Order    uint32 `json:"order"`
}

type reorderRequest struct {
	GroupID uint          `json:"group_id"`
	Items   []reorderItem `json:"items"`
}

// POST /admin/menus/reorder — persists the tree's structure (parent + order)
// after a drag-and-drop move. All items must belong to the submitted group.
func AdminReorderMenus(c echo.Context) error {
	var req reorderRequest
	if err := c.Bind(&req); err != nil || req.GroupID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	ids := make([]uint, 0, len(req.Items))
	parentByID := map[uint]uint{}
	for _, item := range req.Items {
		if item.ParentID == item.ID {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "a menu cannot be its own parent"})
		}
		ids = append(ids, item.ID)
		parentByID[item.ID] = item.ParentID
	}
	if len(ids) == 0 {
		return c.JSON(http.StatusOK, map[string]bool{"ok": true})
	}

	// Every non-root parent must itself be one of the items being reordered
	// (i.e. actually belong to this group) — guards against cross-group
	// reparenting from a tampered request.
	inSet := make(map[uint]bool, len(ids))
	for _, id := range ids {
		inSet[id] = true
	}
	for _, parentID := range parentByID {
		if parentID != 0 && !inSet[parentID] {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid parent"})
		}
	}

	var count int64
	db.DB.Model(&models.Menu{}).Where("id IN ? AND menu_group_id = ?", ids, req.GroupID).Count(&count)
	if int(count) != len(ids) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "items do not match menu group"})
	}

	tx := db.DB.Begin()
	for _, item := range req.Items {
		if err := tx.Model(&models.Menu{}).Where("id = ?", item.ID).
			Updates(map[string]interface{}{"parent_id": item.ParentID, "list_order": item.Order}).Error; err != nil {
			tx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save order"})
		}
	}
	tx.Commit()

	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}
