package client

import (
	"net/url"

	"github.com/maliceio/engine/api/types/plugin"
	"golang.org/x/net/context"
)

// PluginRemove removes a plugin
func (cli *Client) PluginRemove(ctx context.Context, name string, options plugin.RemoveOptions) error {
	query := url.Values{}
	if options.Force {
		query.Set("force", "1")
	}

	resp, err := cli.delete(ctx, "/plugins/"+name, query, nil)
	ensureReaderClosed(resp)
	return wrapResponseError(err, resp, "plugin", name)
}
