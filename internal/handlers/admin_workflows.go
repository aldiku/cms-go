package handlers

import (
	"encoding/json"
	"net/http"

	"cms-go/internal/db"
	"cms-go/internal/models"

	"github.com/labstack/echo/v4"
)

// workflowStepView adds a decoded Config map (for rendering as key/value
// rows) and, for the "service" step type, a live snapshot of the currently
// active PaymentGateway(s) — so the builder view reflects what's actually
// configured right now, not just what was true when the workflow was seeded.
type workflowStepView struct {
	models.WorkflowStep
	Config         map[string]string
	ActiveGateways []models.PaymentGateway
}

// GET /admin/workflows
func AdminWorkflows(c echo.Context) error {
	var workflows []models.Workflow
	db.DB.Order("name asc").Find(&workflows)

	counts := map[uint]int64{}
	for _, wf := range workflows {
		var n int64
		db.DB.Model(&models.WorkflowStep{}).Where("workflow_id = ?", wf.ID).Count(&n)
		counts[wf.ID] = n
	}

	data := map[string]interface{}{
		"Workflows":  workflows,
		"StepCounts": counts,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/workflows.html", data)
}

// GET /admin/workflows/:id — the "workflow builder" detail view: renders the
// workflow's steps as a connected flow, decoding each step's ConfigJSON and,
// for service steps, enriching with the live PaymentGateway(s) actually
// wired up right now.
func AdminWorkflowDetail(c echo.Context) error {
	var wf models.Workflow
	if err := db.DB.First(&wf, c.Param("id")).Error; err != nil {
		return c.String(http.StatusNotFound, "Workflow not found")
	}

	var rawSteps []models.WorkflowStep
	db.DB.Where("workflow_id = ?", wf.ID).Order("step_order asc").Find(&rawSteps)

	var activeGateways []models.PaymentGateway
	db.DB.Where("status = 1").Order("id asc").Find(&activeGateways)

	steps := make([]workflowStepView, 0, len(rawSteps))
	for _, s := range rawSteps {
		view := workflowStepView{WorkflowStep: s, Config: map[string]string{}}
		if s.ConfigJSON != "" {
			json.Unmarshal([]byte(s.ConfigJSON), &view.Config)
		}
		if s.StepType == models.WorkflowStepService {
			view.ActiveGateways = activeGateways
		}
		steps = append(steps, view)
	}

	data := map[string]interface{}{
		"Workflow": wf,
		"Steps":    steps,
	}
	return renderWithLayout(c, "internal/views/admin/admin-layout.html", "internal/views/admin/workflow_detail.html", data)
}
