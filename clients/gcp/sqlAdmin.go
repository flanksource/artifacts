package gcp

import (
	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/context"
	"google.golang.org/api/option"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

func NewSQLAdmin(ctx context.Context, conn *connection.GCPConnection) (*sqladmin.Service, error) {
	conn = conn.Validate()
	var err error
	var client *sqladmin.Service
	if !conn.Credentials.IsEmpty() {
		credential, err := ctx.GetEnvValueFromCache(*conn.Credentials, ctx.GetNamespace())
		if err != nil {
			return nil, err
		}
		client, err = sqladmin.NewService(ctx.Context, option.WithEndpoint(conn.Endpoint), option.WithCredentialsJSON([]byte(credential)))
		if err != nil {
			return nil, err
		}
	} else {
		client, err = sqladmin.NewService(ctx.Context, option.WithEndpoint(conn.Endpoint))
		if err != nil {
			return nil, err
		}
	}
	return client, nil
}
