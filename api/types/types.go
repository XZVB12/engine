package types

// Version contains response of Engine API:
// GET "/version"
type Version struct {
	Platform struct{ Name string } `json:",omitempty"`
	// Components []ComponentVersion    `json:",omitempty"`

	// The following fields are deprecated, they relate to the Engine component and are kept for backwards compatibility

	Version       string
	APIVersion    string `json:"ApiVersion"`
	MinAPIVersion string `json:"MinAPIVersion,omitempty"`
	GitCommit     string
	GoVersion     string
	Os            string
	Arch          string
	KernelVersion string `json:",omitempty"`
	Experimental  bool   `json:",omitempty"`
	BuildTime     string `json:",omitempty"`
}

// Ping contains response of Engine API:
// GET "/_ping"
type Ping struct {
	APIVersion   string
	OSType       string
	Experimental bool
}
