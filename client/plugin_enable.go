package client

import (
	"net/url"
	"strconv"

	"github.com/maliceio/engine/api/types/plugin"
	"golang.org/x/net/context"
)

// PluginEnable enables a plugin
func (cli *Client) PluginEnable(ctx context.Context, name string, options plugin.EnableOptions) error {
	query := url.Values{}
	query.Set("timeout", strconv.Itoa(options.Timeout))

	resp, err := cli.post(ctx, "/plugins/"+name+"/enable", query, nil, nil)
	ensureReaderClosed(resp)
	return err
}
