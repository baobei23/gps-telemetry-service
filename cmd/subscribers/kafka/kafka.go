// Package kafka implements the Kafka subscription functionality
package kafka

import "gps-telemetry-service/internal/users"

type Kafka struct {
	users *users.Users
}

func New(userSvc *users.Users) *Kafka {
	return &Kafka{
		users: userSvc,
	}
}
