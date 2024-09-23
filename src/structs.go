package main

import "time"

type Attributes struct {
	AlbumName  string `json:"albumName"`
	ArtistName string `json:"artistName"`
	Artwork    struct {
		Width  int    `json:"width"`
		Height int    `json:"height"`
		URL    string `json:"url"`
	} `json:"artwork"`
	ComposerName     string   `json:"composerName"`
	DiscNumber       int      `json:"discNumber"`
	DurationInMillis int      `json:"durationInMillis"`
	GenreNames       []string `json:"genreNames"`
	Isrc             string   `json:"isrc"`
	Name             string   `json:"name"`
	PlayParams       struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	} `json:"playParams"`
	Previews []struct {
		URL string `json:"url"`
	} `json:"previews"`
	ReleaseDate time.Time `json:"releaseDate"`
	TrackNumber int       `json:"trackNumber"`
	SongID      string    `json:"songId"`
	Kind        string    `json:"kind"`
	Status      bool      `json:"status"`
	URL         struct {
		Cider      string `json:"cider"`
		AppleMusic string `json:"appleMusic"`
		SongLink   string `json:"songLink"`
	} `json:"url"`
	RemainingTime           float64 `json:"remainingTime"`
	CurrentPlaybackTime     float64 `json:"currentPlaybackTime"`
	CurrentPlaybackProgress float64 `json:"currentPlaybackProgress"`
	StartTime               float64 `json:"startTime"`
	EndTime                 int64   `json:"endTime"`
}

type RpcOptions struct {
	Paused  bool   `json:"paused"`
	Enabled bool   `json:"enabled"`
	Client  string `json:"client"`
	Buttons []struct {
		Label string `json:"label"`
		Url   string `json:"url"`
	} `json:"buttons"`
}

type PluginMetadata struct {
	Name               string   `json:"name"`
	Version            string   `json:"version"`
	Description        string   `json:"description"`
	Authors            []string `json:"authors"`
	FrontendMainScript string   `json:"FrontendMainScript" json:"-"`
	BackendMainScript  string   `json:"BackendMainScript" json:"-"`
}
