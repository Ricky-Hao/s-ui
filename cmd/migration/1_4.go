package migration

import (
	"github.com/alireza0/s-ui/database/model"

	"gorm.io/gorm"
)

func to1_4(tx *gorm.DB) error {
	// Create client_autoresets table
	if !tx.Migrator().HasTable(&model.ClientAutoreset{}) {
		err := tx.Migrator().CreateTable(&model.ClientAutoreset{})
		if err != nil {
			return err
		}
	}

	// Create traffic_histories table
	if !tx.Migrator().HasTable(&model.TrafficHistory{}) {
		err := tx.Migrator().CreateTable(&model.TrafficHistory{})
		if err != nil {
			return err
		}
	}

	return nil
}
