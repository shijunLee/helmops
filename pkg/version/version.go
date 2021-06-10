package version

const (
	version = "0.0.1"
)

var (
	Tag       = ""
	Branch    = ""
	CommitID  = ""
	BuildTime = ""
	Version   = ""
)

//Info pplication build info and version info
type Info struct {
	Version   string `json:"version"`
	Tag       string `json:"tag"`
	Branch    string `json:"branch"`
	CommitID  string `json:"commit_id"`
	BuildTime string `json:"build_time"`
}

//GetVersion get application build info and version info
func GetVersion() *Info {
	appVersion := Version
	if appVersion == "" {
		appVersion = version
	}
	return &Info{
		Version:   appVersion,
		Tag:       Tag,
		Branch:    Branch,
		CommitID:  CommitID,
		BuildTime: BuildTime,
	}
}
