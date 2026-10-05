package telemetry

import (
	"encoding/base64"
	"errors"
	"strconv"
	"time"
)

var ErrInvalidCursor = errors.New("invalid pagination cursor")

// EncodeCursor encodes a device timestamp into a base64 cursor token.
func EncodeCursor(t time.Time) string {
	raw := strconv.FormatInt(t.UnixNano(), 10)
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor decodes a base64 cursor token into a device timestamp.
func DecodeCursor(cursor string) (time.Time, error) {
	data, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, ErrInvalidCursor
	}

	nano, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return time.Time{}, ErrInvalidCursor
	}

	return time.Unix(0, nano).UTC(), nil
}
