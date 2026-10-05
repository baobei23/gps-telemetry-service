package telemetry

import "time"

// Device represents a tracked GPS device.
type Device struct {
	ID          string    `json:"id"`
	FirstSeenAt time.Time `json:"firstSeenAt"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// DeviceSummary represents a device item in the list view with activity info.
type DeviceSummary struct {
	ID          string    `json:"id"`
	FirstSeenAt time.Time `json:"firstSeenAt"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
	Status      string    `json:"status"` // "active" or "inactive"
}

// PositionsFilter specifies query parameters for position history.
type PositionsFilter struct {
	DeviceID   string
	From       *time.Time
	To         *time.Time
	Limit      int
	CursorTime *time.Time
}

// PaginatedPositions holds the slice of positions and the pagination cursor.
type PaginatedPositions struct {
	Positions  []Position `json:"positions"`
	NextCursor *string    `json:"nextCursor,omitempty"`
}

// Position represents a single GPS fix reported by a device.
type Position struct {
	ID         int64     `json:"id"`
	DeviceID   string    `json:"deviceId"`
	DeviceTime time.Time `json:"deviceTime"`
	ServerTime time.Time `json:"serverTime"`
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Valid      int16     `json:"valid"`
	Speed      *float64  `json:"speed,omitempty"`
	Bearing    *float64  `json:"bearing,omitempty"`
	Altitude   *float64  `json:"altitude,omitempty"`
	Accuracy   *float64  `json:"accuracy,omitempty"`
	HDOP       *float64  `json:"hdop,omitempty"`
	Batt       *float64  `json:"batt,omitempty"`
	Charge     *bool     `json:"charge,omitempty"`
}
