package plugin

import (
	"io"
	"net/http"

	"github.com/docker/distribution/reference"
	"github.com/maliceio/engine/api/types"
	"github.com/maliceio/engine/api/types/filters"
	"github.com/maliceio/engine/plugin"
	"golang.org/x/net/context"
)

// Backend for Plugin
type Backend interface {
	Disable(name string, config *types.PluginDisableConfig) error
	Enable(name string, config *types.PluginEnableConfig) error
	List(filters.Args) ([]types.Plugin, error)
	Inspect(name string) (*types.Plugin, error)
	Remove(name string, config *types.PluginRmConfig) error
	Set(name string, args []string) error
	Privileges(ctx context.Context, ref reference.Named, metaHeaders http.Header, authConfig *types.AuthConfig) (types.PluginPrivileges, error)
	Pull(ctx context.Context, ref reference.Named, name string, metaHeaders http.Header, authConfig *types.AuthConfig, privileges types.PluginPrivileges, outStream io.Writer, opts ...plugin.CreateOpt) error
	Push(ctx context.Context, name string, metaHeaders http.Header, authConfig *types.AuthConfig, outStream io.Writer) error
	Upgrade(ctx context.Context, ref reference.Named, name string, metaHeaders http.Header, authConfig *types.AuthConfig, privileges types.PluginPrivileges, outStream io.Writer) error
	CreateFromContext(ctx context.Context, tarCtx io.ReadCloser, options *types.PluginCreateOptions) error
}
