package cli

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func versionString() string {
	return version + " (commit " + commit + ", built " + date + ")"
}
