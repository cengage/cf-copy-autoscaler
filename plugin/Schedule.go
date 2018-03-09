package plugin

import (
	"time"
)

type Schedule struct {
	Resources []struct {
		ExecutesAt         time.Time `json:"executes_at"`
		MinInstances       int       `json:"min_instances"`
		MaxInstances       int       `json:"max_instances"`
		Recurrence         int       `json:"recurrence"`
		Enabled            bool      `json:"enabled"`
	} `json:"resources"`
}