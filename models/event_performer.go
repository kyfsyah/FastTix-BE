package models

type EventPerformer struct {
	ID       int  `gorm:"primaryKey;autoIncrement" json:"id"`
	EventID  int  `gorm:"column:event_id;not null" json:"event_id"`
	SingerID *int `gorm:"column:singer_id" json:"singer_id"`
	BandID   *int `gorm:"column:band_id" json:"band_id"`

	Singer *Singer `gorm:"foreignKey:SingerID;references:ID" json:"singer,omitempty"`
	Band   *Band   `gorm:"foreignKey:BandID;references:ID" json:"band,omitempty"`
}

func (EventPerformer) TableName() string {
	return "event_performers"
}

type PerformerItemRequest struct {
	Type     string `json:"type"`      // "singer" atau "band"
	ID       int    `json:"id"`        // ID jika menggunakan type
	SingerID *int   `json:"singer_id"` // atau langsung singer_id
	BandID   *int   `json:"band_id"`   // atau langsung band_id
}
