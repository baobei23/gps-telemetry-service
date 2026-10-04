package telemetry

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	ErrMissingDeviceID  = errors.New("missing or empty required parameter: id")
	ErrInvalidDeviceID  = errors.New("invalid parameter id: max length is 32 characters")
	ErrMissingTimestamp = errors.New("missing required parameter: timestamp")
	ErrInvalidTimestamp = errors.New("invalid parameter timestamp: must be valid epoch seconds")
	ErrMissingLatitude  = errors.New("missing required parameter: lat")
	ErrInvalidLatitude  = errors.New("invalid parameter lat: must be decimal degrees between -90 and 90")
	ErrMissingLongitude = errors.New("missing required parameter: lon")
	ErrInvalidLongitude = errors.New("invalid parameter lon: must be decimal degrees between -180 and 180")
	ErrInvalidValid     = errors.New("invalid parameter valid: must be 0 or 1")
	ErrInvalidSpeed     = errors.New("invalid parameter speed: must be a non-negative number")
	ErrInvalidBearing   = errors.New("invalid parameter bearing: must be between 0 and 360 degrees")
	ErrInvalidAltitude  = errors.New("invalid parameter altitude: must be a valid number")
	ErrInvalidAccuracy  = errors.New("invalid parameter accuracy: must be a non-negative number")
	ErrInvalidHDOP      = errors.New("invalid parameter hdop: must be a non-negative number")
	ErrInvalidBatt      = errors.New("invalid parameter batt: must be between 0 and 100")
	ErrInvalidCharge    = errors.New("invalid parameter charge: must be a boolean (true/false)")
)

func parseFiniteFloat(s string) (float64, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, errors.New("not a finite number")
	}
	return v, nil
}

// ParseOsmAnd parses and validates query parameters from an OsmAnd HTTP GET request.
func ParseOsmAnd(values url.Values, serverTime time.Time) (*Position, error) {
	id := strings.TrimSpace(values.Get("id"))
	if id == "" {
		return nil, ErrMissingDeviceID
	}
	if len(id) > 32 {
		return nil, ErrInvalidDeviceID
	}

	tsStr := strings.TrimSpace(values.Get("timestamp"))
	if tsStr == "" {
		return nil, ErrMissingTimestamp
	}
	tsUnix, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil || tsUnix <= 0 {
		return nil, ErrInvalidTimestamp
	}
	deviceTime := time.Unix(tsUnix, 0).UTC()

	latStr := strings.TrimSpace(values.Get("lat"))
	if latStr == "" {
		return nil, ErrMissingLatitude
	}
	lat, err := parseFiniteFloat(latStr)
	if err != nil || lat < -90.0 || lat > 90.0 {
		return nil, ErrInvalidLatitude
	}

	lonStr := strings.TrimSpace(values.Get("lon"))
	if lonStr == "" {
		return nil, ErrMissingLongitude
	}
	lon, err := parseFiniteFloat(lonStr)
	if err != nil || lon < -180.0 || lon > 180.0 {
		return nil, ErrInvalidLongitude
	}

	pos := &Position{
		DeviceID:   id,
		DeviceTime: deviceTime,
		ServerTime: serverTime.UTC(),
		Lat:        lat,
		Lon:        lon,
		Valid:      1,
	}

	if valStr := strings.TrimSpace(values.Get("valid")); valStr != "" {
		v, err := strconv.ParseInt(valStr, 10, 16)
		if err != nil || (v != 0 && v != 1) {
			return nil, ErrInvalidValid
		}
		pos.Valid = int16(v)
	}

	if sStr := strings.TrimSpace(values.Get("speed")); sStr != "" {
		s, err := parseFiniteFloat(sStr)
		if err != nil || s < 0 {
			return nil, ErrInvalidSpeed
		}
		pos.Speed = &s
	}

	if bStr := strings.TrimSpace(values.Get("bearing")); bStr != "" {
		b, err := parseFiniteFloat(bStr)
		if err != nil || b < 0 || b > 360 {
			return nil, ErrInvalidBearing
		}
		pos.Bearing = &b
	}

	if aStr := strings.TrimSpace(values.Get("altitude")); aStr != "" {
		a, err := parseFiniteFloat(aStr)
		if err != nil {
			return nil, ErrInvalidAltitude
		}
		pos.Altitude = &a
	}

	if accStr := strings.TrimSpace(values.Get("accuracy")); accStr != "" {
		acc, err := parseFiniteFloat(accStr)
		if err != nil || acc < 0 {
			return nil, ErrInvalidAccuracy
		}
		pos.Accuracy = &acc
	}

	if hStr := strings.TrimSpace(values.Get("hdop")); hStr != "" {
		h, err := parseFiniteFloat(hStr)
		if err != nil || h < 0 {
			return nil, ErrInvalidHDOP
		}
		pos.HDOP = &h
	}

	if battStr := strings.TrimSpace(values.Get("batt")); battStr != "" {
		batt, err := parseFiniteFloat(battStr)
		if err != nil || batt < 0 || batt > 100 {
			return nil, ErrInvalidBatt
		}
		pos.Batt = &batt
	}

	if cStr := strings.TrimSpace(values.Get("charge")); cStr != "" {
		c, err := strconv.ParseBool(cStr)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidCharge, cStr)
		}
		pos.Charge = &c
	}

	return pos, nil
}
