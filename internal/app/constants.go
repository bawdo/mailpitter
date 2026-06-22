package app

// Container and image identity. Bump Image deliberately; reference the
// mailpit changelog before changing.
const (
	ContainerName = "mailpitter"
	Image         = "axllent/mailpit:v1.21"
	DataVolume    = "mailpitter_data"
	// LatestImage is the moving tag `upgrade` pulls. Distinct from the
	// pinned Image used by install.
	LatestImage = "axllent/mailpit:latest"
)

// Network constants.
const (
	HostBindAddr = "127.0.0.1"
	SMTPPort     = 1025
	UIPort       = 8025
	Domain       = "mailpit.test"
)

// UIBaseURL is the loopback URL the mailpit admin API listens on. Used
// by the mailpitapi client when running on the host.
const UIBaseURL = "http://127.0.0.1:8025"
