package domain

type Stream struct {
	ID            int
	Title         string
	Description   string
	Path          string
	InitialOffset int
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
