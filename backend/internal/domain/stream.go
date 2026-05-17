package domain

import "time"

type Stream struct {
	ID          int
	Title       string
	Description string
	Path        string
	StartedAt   time.Time
}

type Segment struct {
	Name     string
	Duration float64
	Position int
}

type PlaylistSnapshot struct {
	Segments       []Segment
	MediaSequence  int
	TargetDuration int
}
