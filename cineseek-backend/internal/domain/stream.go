package domain

import (
	"strings"
	"time"
)

type StreamType string

const (
	StreamTypeMagnet StreamType = "magnet"
	StreamTypeDirect StreamType = "direct"
	StreamTypeHLS    StreamType = "hls"
)

type Stream struct {
	ID          string     `json:"id"`
	MediaID     string     `json:"media_id,omitempty"`
	Provider    string     `json:"provider"`
	Title       string     `json:"title"`
	URL         string     `json:"url"`
	Quality     string     `json:"quality,omitempty"`
	Type        StreamType `json:"type"`
	Seeders     int        `json:"seeders,omitempty"`
	Leechers    int        `json:"leechers,omitempty"`
	IsAlive     bool       `json:"is_alive"`
	LastChecked time.Time  `json:"last_checked,omitempty"`
}

// InferType inspects the URL string and returns the matching StreamType.
func (s *Stream) InferType() StreamType {
	switch {
	case strings.HasPrefix(s.URL, "magnet:"):
		return StreamTypeMagnet
	case strings.Contains(s.URL, ".m3u8"):
		return StreamTypeHLS
	default:
		return StreamTypeDirect
	}
}
