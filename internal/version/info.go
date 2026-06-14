package version

// Populated at build time via -ldflags "-X scheduler/internal/version.Version=..."
var (
	Version   = "dev"
	BuildTime = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
}

func Get() Info {
	return Info{
		Version:   Version,
		BuildTime: BuildTime,
	}
}
