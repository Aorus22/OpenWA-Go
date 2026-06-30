package models

import "time"

type Settings struct {
	Key       string    `gorm:"primaryKey;type:varchar(255)" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}
