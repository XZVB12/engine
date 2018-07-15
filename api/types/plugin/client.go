package plugin

// RemoveOptions holds parameters to remove plugins.
type RemoveOptions struct {
	Force bool
}

// EnableOptions holds parameters to enable plugins.
type EnableOptions struct {
	Timeout int
}

// DisableOptions holds parameters to disable plugins.
type DisableOptions struct {
	Force bool
}

// InstallOptions holds parameters to install a plugin.
type InstallOptions struct {
	Disabled             bool
	AcceptAllPermissions bool
	RegistryAuth         string // RegistryAuth is the base64 encoded credentials for the registry
	RemoteRef            string // RemoteRef is the plugin name on the registry
	// PrivilegeFunc         RequestPrivilegeFunc
	AcceptPermissionsFunc func(Privileges) (bool, error)
	Args                  []string
}
