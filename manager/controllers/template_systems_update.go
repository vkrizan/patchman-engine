package controllers

import (
	"app/base/database"
	"app/base/models"
	"app/base/utils"
	"app/manager/config"
	"app/manager/middlewares"
	"fmt"
	"net/http"

	errors2 "errors"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
)

type TemplateSystemsUpdateRequest struct {
	// List of inventory IDs to have templates removed
	Systems []string `json:"systems" example:"system1-uuid, system2-uuid, ..."`
}

// @Summary Add systems to a template
// @Description Add systems to a template
// @ID updateTemplateSystems
// @Security RhIdentity
// @Accept   json
// @Produce  json
// @Param    body    body   TemplateSystemsUpdateRequest true "Request body"
// @Param    template_id    path  string   true  "Template ID"
// @Success 200
// @Failure 400 {object} 	utils.ErrorResponse
// @Failure 404 {object} 	utils.ErrorResponse
// @Failure 500 {object} 	utils.ErrorResponse
// @Router /templates/{template_id}/systems [PUT]
func TemplateSystemsUpdateHandler(c *gin.Context) {
	account := c.GetInt(utils.KeyAccount)
	templateUUID := c.Param("template_id")
	groups := c.GetStringMapString(utils.KeyInventoryGroups)

	var req TemplateSystemsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		LogAndRespBadRequest(c, err, "Invalid template update request "+err.Error())
		return
	}

	db := middlewares.DBFromContext(c)
	template, err := getTemplate(c, db, account, templateUUID)
	if err != nil {
		// respose set in getTemplateID()
		return
	}

	err = assignTemplateSystems(c, db, account, template, req.Systems, groups)
	if err != nil {
		return
	}

	// TODO: re-evaluate systems added/removed from templates
	// inventoryAIDs := kafka.InventoryIDs2InventoryAIDs(account, req.InventoryIDs)
	// kafka.EvaluateBaselineSystems(inventoryAIDs)

	c.Status(http.StatusOK)
}

func assignTemplateSystems(c *gin.Context, db *gorm.DB, accountID int, template *models.Template,
	inventoryIDs []string, groups map[string]string) error {
	if len(inventoryIDs) == 0 {
		err := errors.New(InvalidInventoryIDsErr)
		LogAndRespBadRequest(c, err, InvalidInventoryIDsErr)
		return err
	}
	tx := db.Begin()
	defer tx.Rollback()

	missingIDs, satelliteManagedIDs, err := checkInventoryIDs(db, accountID, inventoryIDs, groups)
	if err != nil {
		LogAndRespError(c, err, "Database error")
		return err
	}

	if config.EnableSatelliteFunctionality && len(satelliteManagedIDs) > 0 {
		msg := fmt.Sprintf("Template can not contain satellite managed systems: %v", satelliteManagedIDs)
		LogAndRespBadRequest(c, errors.New(msg), msg)
		return err
	} else if len(missingIDs) > 0 {
		msg := fmt.Sprintf("Unknown inventory_ids: %v", missingIDs)
		LogAndRespNotFound(c, errors.New(msg), msg)
		return err
	}

	if err := templateArchVersionMatch(db, inventoryIDs, template, accountID, groups); err != nil {
		msg := fmt.Sprintf("Incompatible template and system version or architecture: %s", err.Error())
		LogAndRespBadRequest(c, err, msg)
		return err
	}

	// if we want to unassign system from template, we need to set template_id=null
	var templateID *int64
	if template != nil && template.ID != 0 {
		templateID = &template.ID
	}

	tx = tx.Model(models.SystemPlatform{}).
		Where("rh_account_id = ? AND inventory_id IN (?::uuid)",
			accountID, inventoryIDs).
		Update("template_id", templateID)
	if e := tx.Error; e != nil {
		LogAndRespError(c, err, "Database error")
		return e
	}
	if int(tx.RowsAffected) != len(inventoryIDs) {
		err = errors.New(InvalidInventoryIDsErr)
		LogAndRespBadRequest(c, err, InvalidInventoryIDsErr)
		return err
	}

	err = tx.Commit().Error
	if e := tx.Error; e != nil {
		LogAndRespError(c, err, "Database error")
		return err
	}
	return nil
}

func templateArchVersionMatch(
	db *gorm.DB, inventoryIDs []string, template *models.Template, acc int, groups map[string]string,
) error {
	var sysArchVersions = []struct {
		InventoryID string
		Arch        string
		Version     string
	}{}
	var err error
	err = database.Systems(db, acc, groups).
		Select("ih.id as inventory_id, ih.system_profile->'operating_system'->>'major' as version, sp.arch as arch").
		Where("ih.id in (?)", inventoryIDs).Find(&sysArchVersions).Error
	if err != nil {
		return err
	}

	for _, sys := range sysArchVersions {
		if sys.Version != template.Version || sys.Arch != template.Arch {
			systemErr := fmt.Errorf("system uuid: %s, arch: %s, version: %s", sys.InventoryID, sys.Arch, sys.Version)
			err = errors2.Join(err, systemErr)
		}
	}
	if err != nil {
		err = errors2.Join(fmt.Errorf("template arch: %s, version: %s", template.Arch, template.Version), err)
	}
	return err
}
