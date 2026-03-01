package service

import (
	"encoding/json"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/util/common"
)

type AutoresetService struct{}

// AutoresetData is a helper struct for API data transfer
type AutoresetData struct {
	ResetMode       int `json:"resetMode"`
	ResetDayOfMonth int `json:"resetDayOfMonth"`
	ResetPeriodDays int `json:"resetPeriodDays"`
}

// ToModel converts AutoresetData to model.ClientAutoreset
func (d *AutoresetData) ToModel() *model.ClientAutoreset {
	return &model.ClientAutoreset{
		ResetMode:       d.ResetMode,
		ResetDayOfMonth: d.ResetDayOfMonth,
		ResetPeriodDays: d.ResetPeriodDays,
	}
}

// Get returns the autoreset configuration for a specific client
func (s *AutoresetService) Get(clientId uint) (*model.ClientAutoreset, error) {
	db := database.GetDB()
	var autoreset model.ClientAutoreset
	err := db.Model(model.ClientAutoreset{}).Where("client_id = ?", clientId).First(&autoreset).Error
	if err != nil {
		if database.IsNotFound(err) {
			return nil, nil // Not configured
		}
		return nil, err // Real DB error
	}
	return &autoreset, nil
}

// Save creates or updates the autoreset configuration for a client
// If resetMode is 0 (disabled), the record is deleted
func (s *AutoresetService) Save(clientId uint, data *model.ClientAutoreset) error {
	db := database.GetDB()

	// If reset mode is disabled, delete the record
	if data.ResetMode == model.ResetModeDisabled {
		return db.Where("client_id = ?", clientId).Delete(&model.ClientAutoreset{}).Error
	}

	// Check if record exists
	var existing model.ClientAutoreset
	err := db.Where("client_id = ?", clientId).First(&existing).Error

	data.ClientId = clientId

	if err != nil {
		// Record doesn't exist, create new
		if data.CreatedAt == 0 {
			data.CreatedAt = time.Now().Unix()
		}
		return db.Create(data).Error
	}

	// Record exists, update it
	data.Id = existing.Id
	if data.CreatedAt == 0 {
		data.CreatedAt = existing.CreatedAt
	}
	return db.Save(data).Error
}

// Delete removes the autoreset configuration for a client
func (s *AutoresetService) Delete(clientId uint) error {
	db := database.GetDB()
	return db.Where("client_id = ?", clientId).Delete(&model.ClientAutoreset{}).Error
}

// CheckAndResetTraffic checks all clients with autoreset enabled and resets traffic if needed
// Returns the list of inbound IDs that need to be restarted (for re-enabled clients)
func (s *AutoresetService) CheckAndResetTraffic() ([]uint, error) {
	var err error
	var inboundIds []uint

	now := time.Now()
	nowUnix := now.Unix()

	db := database.GetDB()
	tx := db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	// Get all autoreset configurations (only clients with reset enabled are stored)
	var autoresets []model.ClientAutoreset
	err = tx.Model(model.ClientAutoreset{}).Find(&autoresets).Error
	if err != nil {
		return nil, err
	}

	var changes []model.Changes
	var trafficHistories []model.TrafficHistory

	for _, autoreset := range autoresets {
		shouldReset := false
		var periodStartTime int64

		switch autoreset.ResetMode {
		case model.ResetModeMonthly:
			shouldReset, periodStartTime = s.checkMonthlyReset(autoreset, now)
		case model.ResetModePeriodic:
			shouldReset, periodStartTime = s.checkPeriodicReset(autoreset, now)
		}

		if !shouldReset {
			continue
		}

		// Get the client
		var client model.Client
		if e := tx.Model(model.Client{}).Where("id = ?", autoreset.ClientId).First(&client).Error; e != nil {
			logger.Warning("Failed to get client for autoreset: ", e)
			continue
		}

		logger.Debug("Resetting traffic for client: ", client.Name)

		// Record traffic history before reset
		trafficHistories = append(trafficHistories, model.TrafficHistory{
			ClientId:   client.Id,
			ClientName: client.Name,
			StartTime:  periodStartTime,
			EndTime:    nowUnix,
			Up:         client.Up,
			Down:       client.Down,
			ResetMode:  autoreset.ResetMode,
		})

		// Check if client was disabled *only* due to traffic limit (not expired)
		// Only re-enable if: disabled + has volume limit + over limit + not expired
		wasDisabledByTraffic := !client.Enable &&
			client.Volume > 0 &&
			(client.Up+client.Down) >= client.Volume &&
			(client.Expiry == 0 || client.Expiry > nowUnix)

		// Reset traffic on client
		updateData := map[string]interface{}{
			"up":   0,
			"down": 0,
		}

		// Re-enable client if it was disabled due to traffic limit
		if wasDisabledByTraffic {
			updateData["enable"] = true

			// Collect inbound IDs for restart
			var clientInbounds []uint
			if e := json.Unmarshal(client.Inbounds, &clientInbounds); e != nil {
				logger.Warning("Failed to unmarshal client inbounds: ", e)
			} else {
				inboundIds = common.UnionUintArray(inboundIds, clientInbounds)
			}

			nameJson, _ := json.Marshal(client.Name)
			changes = append(changes, model.Changes{
				DateTime: nowUnix,
				Actor:    "ResetTrafficJob",
				Key:      "clients",
				Action:   "enable",
				Obj:      json.RawMessage(nameJson),
			})
		}

		if e := tx.Model(model.Client{}).Where("id = ?", client.Id).Updates(updateData).Error; e != nil {
			logger.Warning("Failed to reset traffic for client ", client.Name, ": ", e)
			continue
		}

		// Update last reset time in autoreset table
		if e := tx.Model(model.ClientAutoreset{}).Where("id = ?", autoreset.Id).
			Update("last_reset_at", nowUnix).Error; e != nil {
			logger.Warning("Failed to update last reset time for client ", client.Name, ": ", e)
			continue
		}

		nameJson, _ := json.Marshal(client.Name)
		changes = append(changes, model.Changes{
			DateTime: nowUnix,
			Actor:    "ResetTrafficJob",
			Key:      "clients",
			Action:   "reset_traffic",
			Obj:      json.RawMessage(nameJson),
		})
	}

	// Save traffic histories
	if len(trafficHistories) > 0 {
		err = tx.Create(&trafficHistories).Error
		if err != nil {
			return nil, err
		}
	}

	// Save changes
	if len(changes) > 0 {
		err = tx.Create(&changes).Error
		if err != nil {
			return nil, err
		}
		LastUpdate = nowUnix
	}

	return inboundIds, nil
}

// checkMonthlyReset checks if a client should have its traffic reset based on monthly schedule
// Returns (shouldReset, periodStartTime)
func (s *AutoresetService) checkMonthlyReset(autoreset model.ClientAutoreset, now time.Time) (bool, int64) {
	today := now.Day()
	currentMonth := now.Month()
	currentYear := now.Year()

	// Determine reset day
	resetDay := autoreset.ResetDayOfMonth
	if resetDay <= 0 {
		// Use creation day as default
		createdAt := time.Unix(autoreset.CreatedAt, 0)
		resetDay = createdAt.Day()
	}

	// Handle months with fewer days
	daysInMonth := time.Date(currentYear, currentMonth+1, 0, 0, 0, 0, 0, now.Location()).Day()
	if resetDay > daysInMonth {
		resetDay = daysInMonth
	}

	// Check if today is the reset day
	if today != resetDay {
		return false, 0
	}

	// Check if already reset this month
	if autoreset.LastResetAt > 0 {
		lastReset := time.Unix(autoreset.LastResetAt, 0)
		if lastReset.Month() == currentMonth && lastReset.Year() == currentYear {
			return false, 0
		}
	}

	// Calculate period start time (last reset or creation time)
	periodStart := autoreset.LastResetAt
	if periodStart == 0 {
		periodStart = autoreset.CreatedAt
	}

	return true, periodStart
}

// checkPeriodicReset checks if a client should have its traffic reset based on N-day period
// Returns (shouldReset, periodStartTime)
func (s *AutoresetService) checkPeriodicReset(autoreset model.ClientAutoreset, now time.Time) (bool, int64) {
	if autoreset.ResetPeriodDays <= 0 {
		return false, 0
	}

	nowUnix := now.Unix()
	periodSeconds := int64(autoreset.ResetPeriodDays) * 24 * 60 * 60

	// Determine the reference point (last reset or creation time)
	referenceTime := autoreset.LastResetAt
	if referenceTime == 0 {
		referenceTime = autoreset.CreatedAt
	}

	// Check if enough time has passed since last reset
	timeSinceReference := nowUnix - referenceTime
	if timeSinceReference < periodSeconds {
		return false, 0
	}

	return true, referenceTime
}

// GetTrafficHistory retrieves paginated traffic history for a specific client
func (s *AutoresetService) GetTrafficHistory(clientId uint, page, pageSize int) ([]model.TrafficHistory, int64, error) {
	var histories []model.TrafficHistory
	var total int64
	db := database.GetDB()

	query := db.Model(model.TrafficHistory{}).Where("client_id = ?", clientId)

	// Get total count
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	offset := (page - 1) * pageSize
	err = query.Order("end_time DESC").Offset(offset).Limit(pageSize).Find(&histories).Error
	if err != nil {
		return nil, 0, err
	}

	return histories, total, nil
}

// GetAllTrafficHistory retrieves all traffic history (for admin)
func (s *AutoresetService) GetAllTrafficHistory(limit int) ([]model.TrafficHistory, error) {
	var histories []model.TrafficHistory
	db := database.GetDB()

	query := db.Model(model.TrafficHistory{}).Order("end_time DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&histories).Error
	return histories, err
}
