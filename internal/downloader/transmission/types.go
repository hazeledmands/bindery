package transmission

// Torrent represents a single torrent as returned by the Transmission RPC API.
type Torrent struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	HashString     string  `json:"hashString"`
	Status         int     `json:"status"`
	PercentDone    float64 `json:"percentDone"`
	RateDownload   int64   `json:"rateDownload"`
	PeersConnected int     `json:"peersConnected"`
	Error          int     `json:"error"`
	ErrorString    string  `json:"errorString"`
	DownloadDir    string  `json:"downloadDir"`
}

// Transmission torrent status constants.
const (
	StatusStopped      = 0
	StatusCheckWait    = 1
	StatusCheck        = 2
	StatusDownloadWait = 3
	StatusDownloading  = 4
	StatusSeedWait     = 5
	StatusSeeding      = 6
)
