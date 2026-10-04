package telemetry

import (
	"errors"
	"net/url"
	"testing"
	"time"
)

func TestParseOsmAnd(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		query     string
		wantErr   error
		checkFunc func(t *testing.T, p *Position)
	}{
		{
			name: "valid full simulator request",
			query: "id=357565514527184&valid=1&timestamp=1700000000&lat=-6.200000&lon=106.816666" +
				"&speed=16.19&bearing=90.0&altitude=12.0&accuracy=5.0&hdop=0.80&batt=87&charge=false",
			wantErr: nil,
			checkFunc: func(t *testing.T, p *Position) {
				if p.DeviceID != "357565514527184" {
					t.Errorf("expected device ID 357565514527184, got %s", p.DeviceID)
				}
				if p.DeviceTime.Unix() != 1700000000 {
					t.Errorf("expected timestamp 1700000000, got %d", p.DeviceTime.Unix())
				}
				if p.Lat != -6.200000 || p.Lon != 106.816666 {
					t.Errorf("unexpected coords: %f, %f", p.Lat, p.Lon)
				}
				if p.Valid != 1 {
					t.Errorf("expected valid=1, got %d", p.Valid)
				}
				if p.Speed == nil || *p.Speed != 16.19 {
					t.Errorf("unexpected speed")
				}
				if p.Bearing == nil || *p.Bearing != 90.0 {
					t.Errorf("unexpected bearing")
				}
				if p.Altitude == nil || *p.Altitude != 12.0 {
					t.Errorf("unexpected altitude")
				}
				if p.Accuracy == nil || *p.Accuracy != 5.0 {
					t.Errorf("unexpected accuracy")
				}
				if p.HDOP == nil || *p.HDOP != 0.80 {
					t.Errorf("unexpected hdop")
				}
				if p.Batt == nil || *p.Batt != 87 {
					t.Errorf("unexpected batt")
				}
				if p.Charge == nil || *p.Charge != false {
					t.Errorf("unexpected charge")
				}
			},
		},
		{
			name:    "missing id",
			query:   "timestamp=1700000000&lat=-6.2&lon=106.8",
			wantErr: ErrMissingDeviceID,
		},
		{
			name:    "id too long",
			query:   "id=12345678901234567890123456789012345&timestamp=1700000000&lat=-6.2&lon=106.8",
			wantErr: ErrInvalidDeviceID,
		},
		{
			name:    "missing timestamp",
			query:   "id=357565514527184&lat=-6.2&lon=106.8",
			wantErr: ErrMissingTimestamp,
		},
		{
			name:    "invalid timestamp string",
			query:   "id=357565514527184&timestamp=invalid&lat=-6.2&lon=106.8",
			wantErr: ErrInvalidTimestamp,
		},
		{
			name:    "missing lat",
			query:   "id=357565514527184&timestamp=1700000000&lon=106.8",
			wantErr: ErrMissingLatitude,
		},
		{
			name:    "lat out of range high",
			query:   "id=357565514527184&timestamp=1700000000&lat=91.0&lon=106.8",
			wantErr: ErrInvalidLatitude,
		},
		{
			name:    "lat out of range low",
			query:   "id=357565514527184&timestamp=1700000000&lat=-91.0&lon=106.8",
			wantErr: ErrInvalidLatitude,
		},
		{
			name:    "missing lon",
			query:   "id=357565514527184&timestamp=1700000000&lat=-6.2",
			wantErr: ErrMissingLongitude,
		},
		{
			name:    "lon out of range",
			query:   "id=357565514527184&timestamp=1700000000&lat=-6.2&lon=185.0",
			wantErr: ErrInvalidLongitude,
		},
		{
			name:    "invalid valid parameter",
			query:   "id=357565514527184&timestamp=1700000000&lat=-6.2&lon=106.8&valid=2",
			wantErr: ErrInvalidValid,
		},
		{
			name:    "negative speed",
			query:   "id=357565514527184&timestamp=1700000000&lat=-6.2&lon=106.8&speed=-5",
			wantErr: ErrInvalidSpeed,
		},
		{
			name:    "bearing out of range",
			query:   "id=357565514527184&timestamp=1700000000&lat=-6.2&lon=106.8&bearing=365",
			wantErr: ErrInvalidBearing,
		},
		{
			name:    "battery out of range",
			query:   "id=357565514527184&timestamp=1700000000&lat=-6.2&lon=106.8&batt=105",
			wantErr: ErrInvalidBatt,
		},
		{
			name:    "invalid charge format",
			query:   "id=357565514527184&timestamp=1700000000&lat=-6.2&lon=106.8&charge=yes",
			wantErr: ErrInvalidCharge,
		},
		{
			name:    "lat NaN rejected",
			query:   "id=357565514527184&timestamp=1700000000&lat=NaN&lon=106.8",
			wantErr: ErrInvalidLatitude,
		},
		{
			name:    "lat Inf rejected",
			query:   "id=357565514527184&timestamp=1700000000&lat=+Inf&lon=106.8",
			wantErr: ErrInvalidLatitude,
		},
		{
			name:    "lon NaN rejected",
			query:   "id=357565514527184&timestamp=1700000000&lat=-6.2&lon=NaN",
			wantErr: ErrInvalidLongitude,
		},
		{
			name:    "speed NaN rejected",
			query:   "id=357565514527184&timestamp=1700000000&lat=-6.2&lon=106.8&speed=NaN",
			wantErr: ErrInvalidSpeed,
		},
		{
			name:    "speed Inf rejected",
			query:   "id=357565514527184&timestamp=1700000000&lat=-6.2&lon=106.8&speed=Inf",
			wantErr: ErrInvalidSpeed,
		},
		{
			name:    "hdop NaN rejected",
			query:   "id=357565514527184&timestamp=1700000000&lat=-6.2&lon=106.8&hdop=NaN",
			wantErr: ErrInvalidHDOP,
		},
		{
			name:    "batt Inf rejected",
			query:   "id=357565514527184&timestamp=1700000000&lat=-6.2&lon=106.8&batt=+Inf",
			wantErr: ErrInvalidBatt,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			values, err := url.ParseQuery(tc.query)
			if err != nil {
				t.Fatalf("failed parsing query: %v", err)
			}
			pos, err := ParseOsmAnd(values, now)
			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tc.wantErr)
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.checkFunc != nil {
				tc.checkFunc(t, pos)
			}
		})
	}
}
