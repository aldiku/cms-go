package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"cms-go/internal/auth"
	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// maxRevisionsPerEntity caps how many revisions are kept per Page/Layout/
// Component. Older revisions beyond this are pruned on every save.
const maxRevisionsPerEntity = 20

// saveRevision snapshots entity (the pre-update state, passed by value) as a
// new Revision row, then trims anything beyond maxRevisionsPerEntity for that
// entityType+entityID pair. Called from the update handlers only — creation
// has no prior state to snapshot.
func saveRevision(c echo.Context, entityType string, entityID uint, entity interface{}) {
	data, err := json.Marshal(entity)
	if err != nil {
		return
	}

	rev := models.Revision{
		EntityType: entityType,
		EntityID:   entityID,
		Data:       string(data),
	}
	if user, ok := c.Get(auth.CtxUser).(models.User); ok {
		rev.UserID = user.ID
		rev.UserName = user.FullName()
	}
	if err := db.DB.Create(&rev).Error; err != nil {
		return
	}

	var staleIDs []uint
	db.DB.Model(&models.Revision{}).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("id desc").
		Offset(maxRevisionsPerEntity).
		Pluck("id", &staleIDs)
	if len(staleIDs) > 0 {
		db.DB.Delete(&models.Revision{}, staleIDs)
	}
}

// loadRevisions fetches the revision log for an entity, newest first, for
// display in a Log tab.
func loadRevisions(entityType string, entityID uint) []models.Revision {
	var revisions []models.Revision
	db.DB.Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("id desc").
		Find(&revisions)
	return revisions
}

// revisionEditURL points "back to the entity" from a revision detail view.
func revisionEditURL(rev models.Revision) string {
	id := strconv.FormatUint(uint64(rev.EntityID), 10)
	switch rev.EntityType {
	case "page":
		return "/admin/pages/" + id + "/edit"
	case "layout":
		return "/admin/layouts/" + id + "/edit"
	case "component":
		return "/admin/components/" + id + "/edit"
	default:
		return "/admin"
	}
}

// revisionField is one row in the revision detail view. IsLong is decided
// here (not in the template) since the raw value is untyped JSON — a string,
// bool, number, or nil depending on the snapshotted field.
type revisionField struct {
	Key    string
	Value  string
	IsLong bool
}

func formatRevisionValue(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case bool:
		return strconv.FormatBool(val)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(b)
	}
}

// GET /admin/revisions/:id — read-only detail view of a single revision
// snapshot, shared across pages, layouts and components.
func AdminViewRevision(c echo.Context) error {
	id := c.Param("id")
	var rev models.Revision
	if err := db.DB.First(&rev, id).Error; err != nil {
		return c.String(http.StatusNotFound, "Revision not found")
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(rev.Data), &raw); err != nil {
		raw = map[string]interface{}{}
	}

	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fields := make([]revisionField, 0, len(keys))
	for _, k := range keys {
		s := formatRevisionValue(raw[k])
		fields = append(fields, revisionField{
			Key:    k,
			Value:  s,
			IsLong: len(s) > 80 || strings.Contains(s, "\n"),
		})
	}

	data := map[string]interface{}{
		"Revision": rev,
		"Fields":   fields,
		"BackURL":  revisionEditURL(rev),
	}

	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/revision_detail.html", data)
}
