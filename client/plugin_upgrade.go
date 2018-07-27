package client

import (
	"io"
	"net/url"

	"github.com/docker/distribution/reference"
	"github.com/maliceio/engine/api/types/plugin"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// PluginUpgrade upgrades a plugin
func (cli *Client) PluginUpgrade(ctx context.Context, name string, options plugin.InstallOptions) (rc io.ReadCloser, err error) {
	if err := cli.NewVersionError("1.26", "plugin upgrade"); err != nil {
		return nil, err
	}
	query := url.Values{}
	if _, err := reference.ParseNormalizedNamed(options.RemoteRef); err != nil {
		return nil, errors.Wrap(err, "invalid remote reference")
	}
	query.Set("remote", options.RemoteRef)

	// privileges, err := cli.checkPluginPermissions(ctx, query, options)
	// if err != nil {
	// 	return nil, err
	// }

	// resp, err := cli.tryPluginUpgrade(ctx, query, privileges, name, options.RegistryAuth)
	resp, err := cli.tryPluginUpgrade(ctx, query, nil, name, options.RegistryAuth)
	if err != nil {
		return nil, err
	}
	return resp.body, nil
}

func (cli *Client) tryPluginUpgrade(ctx context.Context, query url.Values, privileges plugin.Privileges, name, registryAuth string) (serverResponse, error) {
	headers := map[string][]string{"X-Registry-Auth": {registryAuth}}
	return cli.post(ctx, "/plugins/"+name+"/upgrade", query, privileges, headers)
}
