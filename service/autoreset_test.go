package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/op/go-logging"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	logger.InitLogger(logging.DEBUG)
	dbPath := t.TempDir() + "/test.db"
	err := database.InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to init test DB: %v", err)
	}
}

// ─── AutoresetData Tests ────────────────────────────────────────────

func TestAutoResetData_ToModel(t *testing.T) {
	data := &AutoresetData{
		ResetMode:       model.ResetModeMonthly,
		ResetDayOfMonth: 15,
		ResetPeriodDays: 0,
	}
	m := data.ToModel()
	if m.ResetMode != model.ResetModeMonthly {
		t.Errorf("expected ResetMode %d, got %d", model.ResetModeMonthly, m.ResetMode)
	}
	if m.ResetDayOfMonth != 15 {
		t.Errorf("expected ResetDayOfMonth 15, got %d", m.ResetDayOfMonth)
	}
	if m.ResetPeriodDays != 0 {
		t.Errorf("expected ResetPeriodDays 0, got %d", m.ResetPeriodDays)
	}
	// ClientId and Id should be zero (not set by ToModel)
	if m.ClientId != 0 {
		t.Errorf("expected ClientId 0, got %d", m.ClientId)
	}
}

// ─── CRUD Tests ─────────────────────────────────────────────────────

func TestAutoResetService_Get_NotFound(t *testing.T) {
	setupTestDB(t)
	svc := &AutoresetService{}
	result, err := svc.Get(999)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result for non-existent client, got %+v", result)
	}
}

func TestAutoResetService_Get_Exists(t *testing.T) {
	setupTestDB(t)
	db := database.GetDB()
	record := model.ClientAutoreset{
		ClientId:        42,
		ResetMode:       model.ResetModeMonthly,
		ResetDayOfMonth: 10,
		CreatedAt:       time.Now().Unix(),
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("failed to seed: %v", err)
	}

	svc := &AutoresetService{}
	result, err := svc.Get(42)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.ClientId != 42 {
		t.Errorf("expected ClientId 42, got %d", result.ClientId)
	}
	if result.ResetMode != model.ResetModeMonthly {
		t.Errorf("expected ResetMode %d, got %d", model.ResetModeMonthly, result.ResetMode)
	}
}

func TestAutoResetService_Save_Create(t *testing.T) {
	setupTestDB(t)
	svc := &AutoresetService{}
	data := &model.ClientAutoreset{
		ResetMode:       model.ResetModeMonthly,
		ResetDayOfMonth: 5,
	}
	err := svc.Save(100, data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify it was created
	result, err := svc.Get(100)
	if err != nil {
		t.Fatalf("expected no error on get, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result after save, got nil")
	}
	if result.ClientId != 100 {
		t.Errorf("expected ClientId 100, got %d", result.ClientId)
	}
	if result.ResetDayOfMonth != 5 {
		t.Errorf("expected ResetDayOfMonth 5, got %d", result.ResetDayOfMonth)
	}
	if result.CreatedAt == 0 {
		t.Error("expected CreatedAt to be auto-set, got 0")
	}
}

func TestAutoResetService_Save_Update(t *testing.T) {
	setupTestDB(t)
	svc := &AutoresetService{}

	// Create initial
	data := &model.ClientAutoreset{
		ResetMode:       model.ResetModeMonthly,
		ResetDayOfMonth: 5,
	}
	if err := svc.Save(100, data); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Update
	updated := &model.ClientAutoreset{
		ResetMode:       model.ResetModePeriodic,
		ResetPeriodDays: 7,
	}
	if err := svc.Save(100, updated); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// Verify
	result, _ := svc.Get(100)
	if result == nil {
		t.Fatal("expected result after update, got nil")
	}
	if result.ResetMode != model.ResetModePeriodic {
		t.Errorf("expected ResetMode %d, got %d", model.ResetModePeriodic, result.ResetMode)
	}
	if result.ResetPeriodDays != 7 {
		t.Errorf("expected ResetPeriodDays 7, got %d", result.ResetPeriodDays)
	}
}

func TestAutoResetService_Save_Disable(t *testing.T) {
	setupTestDB(t)
	svc := &AutoresetService{}

	// Create first
	data := &model.ClientAutoreset{
		ResetMode:       model.ResetModeMonthly,
		ResetDayOfMonth: 1,
	}
	if err := svc.Save(100, data); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Disable (ResetMode == 0)
	disabled := &model.ClientAutoreset{
		ResetMode: model.ResetModeDisabled,
	}
	if err := svc.Save(100, disabled); err != nil {
		t.Fatalf("disable failed: %v", err)
	}

	// Verify it's gone
	result, _ := svc.Get(100)
	if result != nil {
		t.Errorf("expected nil after disable, got %+v", result)
	}
}

func TestAutoResetService_Delete(t *testing.T) {
	setupTestDB(t)
	svc := &AutoresetService{}

	data := &model.ClientAutoreset{
		ResetMode:       model.ResetModeMonthly,
		ResetDayOfMonth: 1,
	}
	if err := svc.Save(200, data); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if err := svc.Delete(200); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	result, _ := svc.Get(200)
	if result != nil {
		t.Errorf("expected nil after delete, got %+v", result)
	}
}

// ─── checkMonthlyReset Tests ───────────────────────────────────────

func TestCheckMonthlyReset_Today(t *testing.T) {
	svc := &AutoresetService{}
	// Set "now" to January 15
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	createdAt := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC).Unix()

	autoreset := model.ClientAutoreset{
		ResetDayOfMonth: 15,
		ResetMode:       model.ResetModeMonthly,
		CreatedAt:       createdAt,
		LastResetAt:     0,
	}

	shouldReset, periodStart := svc.checkMonthlyReset(autoreset, now)
	if !shouldReset {
		t.Error("expected shouldReset=true on reset day")
	}
	if periodStart != createdAt {
		t.Errorf("expected periodStart=%d (createdAt), got %d", createdAt, periodStart)
	}
}

func TestCheckMonthlyReset_NotToday(t *testing.T) {
	svc := &AutoresetService{}
	now := time.Date(2026, 1, 14, 12, 0, 0, 0, time.UTC) // day 14

	autoreset := model.ClientAutoreset{
		ResetDayOfMonth: 15, // reset on day 15
		ResetMode:       model.ResetModeMonthly,
		CreatedAt:       time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC).Unix(),
	}

	shouldReset, _ := svc.checkMonthlyReset(autoreset, now)
	if shouldReset {
		t.Error("expected shouldReset=false when today is not reset day")
	}
}

func TestCheckMonthlyReset_AlreadyReset(t *testing.T) {
	svc := &AutoresetService{}
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	autoreset := model.ClientAutoreset{
		ResetDayOfMonth: 15,
		ResetMode:       model.ResetModeMonthly,
		CreatedAt:       time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC).Unix(),
		LastResetAt:     time.Date(2026, 1, 15, 1, 0, 0, 0, time.UTC).Unix(), // already reset today
	}

	shouldReset, _ := svc.checkMonthlyReset(autoreset, now)
	if shouldReset {
		t.Error("expected shouldReset=false when already reset this month")
	}
}

func TestCheckMonthlyReset_ShortMonth(t *testing.T) {
	svc := &AutoresetService{}
	// February 2026 has 28 days; reset day 31 should clamp to 28
	now := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)

	autoreset := model.ClientAutoreset{
		ResetDayOfMonth: 31, // requested day 31
		ResetMode:       model.ResetModeMonthly,
		CreatedAt:       time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
	}

	shouldReset, _ := svc.checkMonthlyReset(autoreset, now)
	if !shouldReset {
		t.Error("expected shouldReset=true when reset day clamped to last day of short month")
	}
}

func TestCheckMonthlyReset_DefaultDayFromCreation(t *testing.T) {
	svc := &AutoresetService{}
	// Created on the 20th, resetDayOfMonth is 0 → should use creation day (20)
	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)

	autoreset := model.ClientAutoreset{
		ResetDayOfMonth: 0, // use creation day
		ResetMode:       model.ResetModeMonthly,
		CreatedAt:       time.Date(2025, 6, 20, 0, 0, 0, 0, time.UTC).Unix(),
	}

	shouldReset, _ := svc.checkMonthlyReset(autoreset, now)
	if !shouldReset {
		t.Error("expected shouldReset=true when using default day from creation")
	}
}

// ─── checkPeriodicReset Tests ──────────────────────────────────────

func TestCheckPeriodicReset_DuePeriod(t *testing.T) {
	svc := &AutoresetService{}
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	// 7-day period, check 8 days later
	now := time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)

	autoreset := model.ClientAutoreset{
		ResetPeriodDays: 7,
		ResetMode:       model.ResetModePeriodic,
		CreatedAt:       createdAt,
		LastResetAt:     0,
	}

	shouldReset, ref := svc.checkPeriodicReset(autoreset, now)
	if !shouldReset {
		t.Error("expected shouldReset=true when period has elapsed")
	}
	if ref != createdAt {
		t.Errorf("expected reference=%d (createdAt), got %d", createdAt, ref)
	}
}

func TestCheckPeriodicReset_NotDue(t *testing.T) {
	svc := &AutoresetService{}
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	// 7-day period, check 3 days later
	now := time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)

	autoreset := model.ClientAutoreset{
		ResetPeriodDays: 7,
		ResetMode:       model.ResetModePeriodic,
		CreatedAt:       createdAt,
	}

	shouldReset, _ := svc.checkPeriodicReset(autoreset, now)
	if shouldReset {
		t.Error("expected shouldReset=false when period has not elapsed")
	}
}

func TestCheckPeriodicReset_ZeroPeriod(t *testing.T) {
	svc := &AutoresetService{}
	now := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	autoreset := model.ClientAutoreset{
		ResetPeriodDays: 0,
		ResetMode:       model.ResetModePeriodic,
		CreatedAt:       time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
	}

	shouldReset, _ := svc.checkPeriodicReset(autoreset, now)
	if shouldReset {
		t.Error("expected shouldReset=false when period is 0")
	}
}

func TestCheckPeriodicReset_UsesLastResetAt(t *testing.T) {
	svc := &AutoresetService{}
	lastReset := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC).Unix()
	// 3-day period, check 4 days after last reset
	now := time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)

	autoreset := model.ClientAutoreset{
		ResetPeriodDays: 3,
		ResetMode:       model.ResetModePeriodic,
		CreatedAt:       time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
		LastResetAt:     lastReset,
	}

	shouldReset, ref := svc.checkPeriodicReset(autoreset, now)
	if !shouldReset {
		t.Error("expected shouldReset=true when period elapsed since lastResetAt")
	}
	if ref != lastReset {
		t.Errorf("expected reference=%d (lastResetAt), got %d", lastReset, ref)
	}
}

// ─── CheckAndResetTraffic Integration Tests ────────────────────────

func seedClientWithAutoreset(t *testing.T, clientId uint, clientName string, enable bool, volume, up, down, expiry int64, autoreset model.ClientAutoreset) {
	t.Helper()
	db := database.GetDB()
	client := model.Client{
		Id:       clientId,
		Enable:   enable,
		Name:     clientName,
		Volume:   volume,
		Up:       up,
		Down:     down,
		Expiry:   expiry,
		Inbounds: json.RawMessage(`[1,2]`),
		Config:   json.RawMessage(`{}`),
		Links:    json.RawMessage(`[]`),
	}
	if err := db.Create(&client).Error; err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	autoreset.ClientId = clientId
	if autoreset.CreatedAt == 0 {
		autoreset.CreatedAt = time.Now().Add(-48 * time.Hour).Unix()
	}
	if err := db.Create(&autoreset).Error; err != nil {
		t.Fatalf("failed to create autoreset: %v", err)
	}
}

func TestCheckAndResetTraffic_NoConfigs(t *testing.T) {
	setupTestDB(t)
	svc := &AutoresetService{}
	ids, err := svc.CheckAndResetTraffic()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty inbound IDs, got %v", ids)
	}
}

func TestCheckAndResetTraffic_PeriodicReset(t *testing.T) {
	setupTestDB(t)

	createdAt := time.Now().Add(-48 * time.Hour).Unix() // 2 days ago
	seedClientWithAutoreset(t, 1, "test-client", true, 0, 1000, 2000, 0,
		model.ClientAutoreset{
			ResetMode:       model.ResetModePeriodic,
			ResetPeriodDays: 1, // 1-day period → should reset (created 2 days ago)
			CreatedAt:       createdAt,
		},
	)

	svc := &AutoresetService{}
	_, err := svc.CheckAndResetTraffic()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify traffic was reset
	db := database.GetDB()
	var client model.Client
	db.First(&client, 1)
	if client.Up != 0 || client.Down != 0 {
		t.Errorf("expected traffic reset to 0, got up=%d down=%d", client.Up, client.Down)
	}

	// Verify traffic history was created
	var histories []model.TrafficHistory
	db.Where("client_id = ?", 1).Find(&histories)
	if len(histories) != 1 {
		t.Fatalf("expected 1 traffic history, got %d", len(histories))
	}
	if histories[0].Up != 1000 || histories[0].Down != 2000 {
		t.Errorf("expected history up=1000 down=2000, got up=%d down=%d", histories[0].Up, histories[0].Down)
	}

	// Verify last reset time was updated
	var autoreset model.ClientAutoreset
	db.Where("client_id = ?", 1).First(&autoreset)
	if autoreset.LastResetAt == 0 {
		t.Error("expected LastResetAt to be updated")
	}
}

func TestCheckAndResetTraffic_ReEnableClient(t *testing.T) {
	setupTestDB(t)

	createdAt := time.Now().Add(-48 * time.Hour).Unix()
	// Client is disabled, over volume, not expired
	seedClientWithAutoreset(t, 1, "over-limit-client", false, 1000, 600, 600, 0,
		model.ClientAutoreset{
			ResetMode:       model.ResetModePeriodic,
			ResetPeriodDays: 1,
			CreatedAt:       createdAt,
		},
	)

	svc := &AutoresetService{}
	inboundIds, err := svc.CheckAndResetTraffic()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should have re-enabled and returned inbound IDs
	if len(inboundIds) == 0 {
		t.Error("expected inbound IDs for restart after re-enable")
	}

	// Verify client was re-enabled
	db := database.GetDB()
	var client model.Client
	db.First(&client, 1)
	if !client.Enable {
		t.Error("expected client to be re-enabled")
	}
	if client.Up != 0 || client.Down != 0 {
		t.Errorf("expected traffic reset, got up=%d down=%d", client.Up, client.Down)
	}
}

func TestCheckAndResetTraffic_DontReEnableExpired(t *testing.T) {
	setupTestDB(t)

	createdAt := time.Now().Add(-48 * time.Hour).Unix()
	// Client is disabled, over volume, AND expired
	expiredTime := time.Now().Add(-24 * time.Hour).Unix()
	seedClientWithAutoreset(t, 1, "expired-client", false, 1000, 600, 600, expiredTime,
		model.ClientAutoreset{
			ResetMode:       model.ResetModePeriodic,
			ResetPeriodDays: 1,
			CreatedAt:       createdAt,
		},
	)

	svc := &AutoresetService{}
	inboundIds, err := svc.CheckAndResetTraffic()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should NOT re-enable expired client
	if len(inboundIds) != 0 {
		t.Errorf("expected no inbound IDs for expired client, got %v", inboundIds)
	}

	// Client should still be disabled
	db := database.GetDB()
	var client model.Client
	db.First(&client, 1)
	if client.Enable {
		t.Error("expected expired client to remain disabled")
	}
}

// ─── TrafficHistory Tests ──────────────────────────────────────────

func seedTrafficHistories(t *testing.T, clientId uint, count int) {
	t.Helper()
	db := database.GetDB()
	now := time.Now().Unix()
	for i := 0; i < count; i++ {
		h := model.TrafficHistory{
			ClientId:   clientId,
			ClientName: "test-client",
			StartTime:  now - int64((i+1)*86400),
			EndTime:    now - int64(i*86400),
			Up:         int64(100 * (i + 1)),
			Down:       int64(200 * (i + 1)),
			ResetMode:  model.ResetModePeriodic,
		}
		if err := db.Create(&h).Error; err != nil {
			t.Fatalf("failed to seed history: %v", err)
		}
	}
}

func TestGetTrafficHistory_Pagination(t *testing.T) {
	setupTestDB(t)
	seedTrafficHistories(t, 1, 15)

	svc := &AutoresetService{}

	// Page 1, size 10
	histories, total, err := svc.GetTrafficHistory(1, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 15 {
		t.Errorf("expected total=15, got %d", total)
	}
	if len(histories) != 10 {
		t.Errorf("expected 10 results on page 1, got %d", len(histories))
	}

	// Page 2, size 10
	histories2, total2, err := svc.GetTrafficHistory(1, 2, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total2 != 15 {
		t.Errorf("expected total=15, got %d", total2)
	}
	if len(histories2) != 5 {
		t.Errorf("expected 5 results on page 2, got %d", len(histories2))
	}
}

func TestGetTrafficHistory_Empty(t *testing.T) {
	setupTestDB(t)
	svc := &AutoresetService{}
	histories, total, err := svc.GetTrafficHistory(999, 1, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 0 {
		t.Errorf("expected total=0, got %d", total)
	}
	if len(histories) != 0 {
		t.Errorf("expected 0 results, got %d", len(histories))
	}
}

func TestGetAllTrafficHistory_WithLimit(t *testing.T) {
	setupTestDB(t)
	seedTrafficHistories(t, 1, 10)

	svc := &AutoresetService{}
	histories, err := svc.GetAllTrafficHistory(5)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(histories) != 5 {
		t.Errorf("expected 5 results with limit, got %d", len(histories))
	}
}

func TestGetAllTrafficHistory_NoLimit(t *testing.T) {
	setupTestDB(t)
	seedTrafficHistories(t, 1, 10)

	svc := &AutoresetService{}
	histories, err := svc.GetAllTrafficHistory(0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(histories) != 10 {
		t.Errorf("expected 10 results without limit, got %d", len(histories))
	}
}
